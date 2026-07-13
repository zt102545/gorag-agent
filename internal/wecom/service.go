package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/yourorg/agent_customer_service/internal/db"
	"github.com/yourorg/agent_customer_service/internal/graph/rag"
	"github.com/yourorg/agent_customer_service/internal/knowledge/unanswered"
	wecomsdk "github.com/yourorg/agent_customer_service/internal/wecom/sdk"
)

const singletonConfigID = "default"

type ModelFactory func(context.Context) (model.ToolCallingChatModel, error)

type Service struct {
	db         *gorm.DB
	sdkFactory wecomsdk.Factory
	rag        *rag.Service
	newModel   ModelFactory
	unanswered *unanswered.Service
	codec      secretCodec
	runMu      sync.Mutex
}

func NewService(gdb *gorm.DB, sdkFactory wecomsdk.Factory, ragService *rag.Service, newModel ModelFactory, unansweredService *unanswered.Service, encryptionKeys ...string) *Service {
	key := "development-only-wecom-key"
	if len(encryptionKeys) > 0 && strings.TrimSpace(encryptionKeys[0]) != "" {
		key = encryptionKeys[0]
	}
	return &Service{db: gdb, sdkFactory: sdkFactory, rag: ragService, newModel: newModel, unanswered: unansweredService, codec: newSecretCodec(key)}
}

type ConfigInput struct {
	CorpID                 string   `json:"corp_id"`
	Secret                 string   `json:"secret"`
	PrivateKey             string   `json:"private_key"`
	PublicKeyVersion       int      `json:"public_key_version"`
	ArchiveUserID          string   `json:"archive_user_id"`
	GroupIDs               []string `json:"group_ids"`
	Enabled                bool     `json:"enabled"`
	PollIntervalSeconds    int      `json:"poll_interval_seconds"`
	AnalyzeIntervalSeconds int      `json:"analyze_interval_seconds"`
}

type ConfigView struct {
	ID                     string     `json:"id"`
	CorpID                 string     `json:"corp_id"`
	HasSecret              bool       `json:"has_secret"`
	HasPrivateKey          bool       `json:"has_private_key"`
	PublicKeyVersion       int        `json:"public_key_version"`
	ArchiveUserID          string     `json:"archive_user_id"`
	GroupIDs               []string   `json:"group_ids"`
	Enabled                bool       `json:"enabled"`
	PollIntervalSeconds    int        `json:"poll_interval_seconds"`
	AnalyzeIntervalSeconds int        `json:"analyze_interval_seconds"`
	LastSeq                uint64     `json:"last_seq"`
	LastSyncAt             *time.Time `json:"last_sync_at,omitempty"`
	LastAnalyzeAt          *time.Time `json:"last_analyze_at,omitempty"`
	LastError              string     `json:"last_error,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

func configView(row *db.WeComArchiveConfig) ConfigView {
	return ConfigView{ID: row.ID, CorpID: row.CorpID, HasSecret: strings.TrimSpace(row.Secret) != "", HasPrivateKey: strings.TrimSpace(row.PrivateKey) != "", PublicKeyVersion: row.PublicKeyVersion, ArchiveUserID: row.ArchiveUserID, GroupIDs: []string(row.GroupIDs), Enabled: row.Enabled, PollIntervalSeconds: row.PollIntervalSeconds, AnalyzeIntervalSeconds: row.AnalyzeIntervalSeconds, LastSeq: row.LastSeq, LastSyncAt: row.LastSyncAt, LastAnalyzeAt: row.LastAnalyzeAt, LastError: row.LastError, UpdatedAt: row.UpdatedAt}
}

func (s *Service) config(ctx context.Context) (*db.WeComArchiveConfig, error) {
	var row db.WeComArchiveConfig
	err := s.db.WithContext(ctx).First(&row, "id = ?", singletonConfigID).Error
	return &row, err
}

func (s *Service) GetConfig(ctx context.Context) (ConfigView, error) {
	row, err := s.config(ctx)
	if err == gorm.ErrRecordNotFound {
		return ConfigView{ID: singletonConfigID, PublicKeyVersion: 1, PollIntervalSeconds: 10, AnalyzeIntervalSeconds: 600, GroupIDs: []string{}}, nil
	}
	if err != nil {
		return ConfigView{}, err
	}
	return configView(row), nil
}

func cleanIDs(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (s *Service) SaveConfig(ctx context.Context, in ConfigInput) (ConfigView, error) {
	now := time.Now()
	row, err := s.config(ctx)
	if err != nil && err != gorm.ErrRecordNotFound {
		return ConfigView{}, err
	}
	if err == gorm.ErrRecordNotFound {
		row = &db.WeComArchiveConfig{ID: singletonConfigID, CreatedAt: now}
	}
	row.CorpID = strings.TrimSpace(in.CorpID)
	row.ArchiveUserID = strings.TrimSpace(in.ArchiveUserID)
	row.GroupIDs = db.StringSlice(cleanIDs(in.GroupIDs))
	row.Enabled = in.Enabled
	row.PublicKeyVersion = in.PublicKeyVersion
	if row.PublicKeyVersion <= 0 {
		row.PublicKeyVersion = 1
	}
	row.PollIntervalSeconds = in.PollIntervalSeconds
	if row.PollIntervalSeconds < 5 {
		row.PollIntervalSeconds = 10
	}
	row.AnalyzeIntervalSeconds = in.AnalyzeIntervalSeconds
	if row.AnalyzeIntervalSeconds < 60 {
		row.AnalyzeIntervalSeconds = 600
	}
	if strings.TrimSpace(in.Secret) != "" {
		row.Secret, err = s.codec.encrypt(in.Secret)
		if err != nil {
			return ConfigView{}, err
		}
	}
	if strings.TrimSpace(in.PrivateKey) != "" {
		row.PrivateKey, err = s.codec.encrypt(in.PrivateKey)
		if err != nil {
			return ConfigView{}, err
		}
	}
	if row.Enabled && (row.CorpID == "" || row.Secret == "" || row.PrivateKey == "" || row.ArchiveUserID == "") {
		return ConfigView{}, fmt.Errorf("corp_id, secret, private_key and archive_user_id are required before enabling")
	}
	row.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return ConfigView{}, err
	}
	return configView(row), nil
}

func (s *Service) runtimeConfig(ctx context.Context) (*db.WeComArchiveConfig, error) {
	row, err := s.config(ctx)
	if err != nil {
		return nil, err
	}
	clone := *row
	clone.Secret, err = s.codec.decrypt(row.Secret)
	if err != nil {
		return nil, err
	}
	clone.PrivateKey, err = s.codec.decrypt(row.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &clone, nil
}

func (s *Service) TestConnection(ctx context.Context) error {
	row, err := s.runtimeConfig(ctx)
	if err != nil {
		return err
	}
	client, err := s.sdkFactory.New(row.CorpID, row.Secret)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.Pull(ctx, row.LastSeq, 1)
	return err
}

type SyncResult struct {
	Pulled  int    `json:"pulled"`
	Stored  int    `json:"stored"`
	LastSeq uint64 `json:"last_seq"`
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func messageInScope(msg *wecomsdk.PlainMessage, cfg *db.WeComArchiveConfig) bool {
	if msg.Action != "send" || msg.MsgType != "text" {
		return false
	}
	if msg.From != cfg.ArchiveUserID && !contains(msg.ToList, cfg.ArchiveUserID) {
		return false
	}
	if len(cfg.GroupIDs) > 0 {
		if msg.RoomID == "" || !contains([]string(cfg.GroupIDs), msg.RoomID) {
			return false
		}
	}
	return strings.TrimSpace(msg.Text.Content) != ""
}

func conversationKey(msg db.WeComMessage, employeeID string) string {
	if msg.RoomID != "" {
		return "room:" + msg.RoomID
	}
	peer := msg.FromUserID
	if peer == employeeID {
		peer = ""
		for _, id := range msg.ToUserIDs {
			if id != employeeID {
				peer = id
				break
			}
		}
	}
	return "direct:" + peer
}

func (s *Service) SyncNow(ctx context.Context) (SyncResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	cfg, err := s.runtimeConfig(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	client, err := s.sdkFactory.New(cfg.CorpID, cfg.Secret)
	if err != nil {
		s.setError(ctx, err)
		return SyncResult{}, err
	}
	defer client.Close()
	result := SyncResult{LastSeq: cfg.LastSeq}
	for batch := 0; batch < 100; batch++ {
		records, pullErr := client.Pull(ctx, result.LastSeq, 1000)
		if pullErr != nil {
			s.setError(ctx, pullErr)
			return result, pullErr
		}
		if len(records) == 0 {
			break
		}
		rows := make([]db.WeComMessage, 0, len(records))
		maxSeq := result.LastSeq
		for _, record := range records {
			result.Pulled++
			if record.Seq > maxSeq {
				maxSeq = record.Seq
			}
			if int(record.PublicKeyVersion) != cfg.PublicKeyVersion {
				err = fmt.Errorf("message %s uses public key version %d, configured version is %d", record.MsgID, record.PublicKeyVersion, cfg.PublicKeyVersion)
				s.setError(ctx, err)
				return result, err
			}
			plain, decryptErr := client.Decrypt(ctx, record, cfg.PrivateKey)
			if decryptErr != nil {
				err = fmt.Errorf("decrypt seq %d: %w", record.Seq, decryptErr)
				s.setError(ctx, err)
				return result, err
			}
			if !messageInScope(plain, cfg) {
				continue
			}
			direction := "customer"
			if plain.From == cfg.ArchiveUserID {
				direction = "employee"
			}
			rows = append(rows, db.WeComMessage{ID: uuid.NewString(), Seq: record.Seq, MsgID: plain.MsgID, Action: plain.Action, FromUserID: plain.From, ToUserIDs: db.StringSlice(plain.ToList), RoomID: plain.RoomID, MsgType: plain.MsgType, Content: strings.TrimSpace(plain.Text.Content), Direction: direction, MessageAt: plain.Time(), CreatedAt: time.Now()})
		}
		now := time.Now()
		if txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if len(rows) > 0 {
				res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
				if res.Error != nil {
					return res.Error
				}
				result.Stored += int(res.RowsAffected)
			}
			return tx.Model(&db.WeComArchiveConfig{}).Where("id = ?", singletonConfigID).Updates(map[string]any{"last_seq": maxSeq, "last_sync_at": now, "last_error": "", "updated_at": now}).Error
		}); txErr != nil {
			s.setError(ctx, txErr)
			return result, txErr
		}
		result.LastSeq = maxSeq
		if len(records) < 1000 {
			break
		}
	}
	return result, nil
}

func (s *Service) setError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	_ = s.db.WithContext(ctx).Model(&db.WeComArchiveConfig{}).Where("id = ?", singletonConfigID).Updates(map[string]any{"last_error": err.Error(), "updated_at": time.Now()}).Error
}

type extractedQuestion struct {
	SourceMsgID     string `json:"source_msg_id"`
	Question        string `json:"question"`
	SuggestedAnswer string `json:"suggested_answer"`
}

type extractionResult struct {
	Questions []extractedQuestion `json:"questions"`
}

func parseJSONObject(content string, target any) error {
	content = strings.TrimSpace(content)
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	return json.Unmarshal([]byte(content), target)
}

func tailUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	start := len(value) - maxBytes
	for start < len(value) && !utf8.RuneStart(value[start]) {
		start++
	}
	return value[start:]
}

func (s *Service) extractQuestions(ctx context.Context, employeeID, conversation string) ([]extractedQuestion, error) {
	m, err := s.newModel(ctx)
	if err != nil {
		return nil, err
	}
	prompt := `你是企业微信客服质检助手。下面是一个群聊或单聊的按时间顺序文本记录。
企业客服 userid 是 ` + employeeID + `。只提取客户提出、适合沉淀为标准客服知识库的业务问题，忽略寒暄、感谢、广告和员工自己提出的问题。
如果客服在后续消息中已经回答，将客服原话整理为 suggested_answer；没有回答则留空。source_msg_id 必须使用提出该问题的消息 ID。
只输出 JSON：{"questions":[{"source_msg_id":"...","question":"标准化问题","suggested_answer":"客服已有回答或空字符串"}]}

聊天记录：
` + conversation
	msg, err := m.Generate(ctx, []*schema.Message{schema.SystemMessage("只输出合法 JSON，不要输出 markdown。"), schema.UserMessage(prompt)})
	if err != nil {
		return nil, err
	}
	var out extractionResult
	if err := parseJSONObject(msg.Content, &out); err != nil {
		return nil, fmt.Errorf("parse question extraction: %w", err)
	}
	return out.Questions, nil
}

func normalizeQuestion(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "")
}

func (s *Service) hasKnowledgeMatch(ctx context.Context, question string, hits []rag.SearchHit) (bool, error) {
	for _, hit := range hits {
		if strings.TrimSpace(hit.Answer) != "" && normalizeQuestion(hit.Question) == normalizeQuestion(question) {
			return true, nil
		}
	}
	if len(hits) == 0 {
		return false, nil
	}
	candidates, _ := json.Marshal(hits)
	m, err := s.newModel(ctx)
	if err != nil {
		return false, err
	}
	prompt := fmt.Sprintf("判断客户问题是否已被候选 QA 中任意一条完整回答。语义只是相关但不能完整回答时必须判定 false。只输出 JSON：{\"matched\":true/false}。\n客户问题：%s\n候选 QA：%s", question, candidates)
	msg, err := m.Generate(ctx, []*schema.Message{schema.SystemMessage("只输出合法 JSON。"), schema.UserMessage(prompt)})
	if err != nil {
		return false, err
	}
	var out struct {
		Matched bool `json:"matched"`
	}
	if err := parseJSONObject(msg.Content, &out); err != nil {
		return false, err
	}
	return out.Matched, nil
}

type AnalyzeResult struct {
	RunID           string `json:"run_id"`
	MessageCount    int    `json:"message_count"`
	QuestionCount   int    `json:"question_count"`
	KnownCount      int    `json:"known_count"`
	UnansweredCount int    `json:"unanswered_count"`
}

func (s *Service) AnalyzeDate(ctx context.Context, date string) (AnalyzeResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	cfg, err := s.config(ctx)
	if err != nil {
		return AnalyzeResult{}, err
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if strings.TrimSpace(date) == "" {
		date = time.Now().In(loc).Format("2006-01-02")
	}
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return AnalyzeResult{}, fmt.Errorf("invalid date: %w", err)
	}
	var pending []db.WeComMessage
	if err := s.db.WithContext(ctx).Where("message_at >= ? AND message_at < ? AND analyzed_at IS NULL", day, day.AddDate(0, 0, 1)).Order("message_at asc").Find(&pending).Error; err != nil {
		return AnalyzeResult{}, err
	}
	run := db.WeComAnalysisRun{ID: uuid.NewString(), AnalysisDate: date, Status: "running", StartedAt: time.Now()}
	if err := s.db.WithContext(ctx).Create(&run).Error; err != nil {
		return AnalyzeResult{}, err
	}
	_ = s.db.WithContext(ctx).Model(&db.WeComArchiveConfig{}).Where("id = ?", singletonConfigID).Updates(map[string]any{"last_analyze_at": run.StartedAt, "updated_at": run.StartedAt}).Error
	result := AnalyzeResult{RunID: run.ID, MessageCount: len(pending)}
	if len(pending) == 0 {
		s.finishRun(ctx, &run, result, nil)
		return result, nil
	}
	conversationKeys := map[string]struct{}{}
	for _, msg := range pending {
		conversationKeys[conversationKey(msg, cfg.ArchiveUserID)] = struct{}{}
	}
	var allMessages []db.WeComMessage
	if err := s.db.WithContext(ctx).Where("message_at >= ? AND message_at < ?", day, day.AddDate(0, 0, 1)).Order("message_at asc").Find(&allMessages).Error; err != nil {
		s.finishRun(ctx, &run, result, err)
		return result, err
	}
	grouped := map[string][]db.WeComMessage{}
	for _, msg := range allMessages {
		key := conversationKey(msg, cfg.ArchiveUserID)
		if _, needed := conversationKeys[key]; needed {
			grouped[key] = append(grouped[key], msg)
		}
	}
	for key := range conversationKeys {
		messages := grouped[key]
		if len(messages) > 500 {
			messages = messages[len(messages)-500:]
		}
		var builder strings.Builder
		byID := map[string]db.WeComMessage{}
		for _, msg := range messages {
			byID[msg.MsgID] = msg
			fmt.Fprintf(&builder, "[%s] msg_id=%s sender=%s role=%s content=%s\n", msg.MessageAt.In(loc).Format("15:04:05"), msg.MsgID, msg.FromUserID, msg.Direction, msg.Content)
		}
		contextText := builder.String()
		contextText = tailUTF8(contextText, 24000)
		questions, extractErr := s.extractQuestions(ctx, cfg.ArchiveUserID, contextText)
		if extractErr != nil {
			s.finishRun(ctx, &run, result, extractErr)
			return result, extractErr
		}
		for _, question := range questions {
			question.Question = strings.TrimSpace(question.Question)
			source, ok := byID[question.SourceMsgID]
			if !ok || source.Direction != "customer" || question.Question == "" {
				continue
			}
			result.QuestionCount++
			hits, searchErr := s.rag.SearchHits(ctx, question.Question)
			if searchErr != nil {
				s.finishRun(ctx, &run, result, searchErr)
				return result, searchErr
			}
			matched, matchErr := s.hasKnowledgeMatch(ctx, question.Question, hits)
			if matchErr != nil {
				matched = false
			}
			if matched {
				result.KnownCount++
				continue
			}
			result.UnansweredCount++
			topScore := 0.0
			if len(hits) > 0 {
				topScore = hits[0].Score
			}
			if err := s.unanswered.Capture(ctx, unanswered.CaptureInput{Question: question.Question, SearchQuery: question.Question, Reason: unanswered.ReasonNoHit, HitCount: len(hits), TopScore: topScore, CallerUserID: source.FromUserID, ChannelID: "wecom", TenantID: "wecom", Source: "wecom", SourceRef: source.MsgID, EmployeeUserID: cfg.ArchiveUserID, RoomID: source.RoomID, Context: contextText, DraftAnswer: strings.TrimSpace(question.SuggestedAnswer)}); err != nil {
				s.finishRun(ctx, &run, result, err)
				return result, err
			}
		}
	}
	now := time.Now()
	ids := make([]string, 0, len(pending))
	for _, row := range pending {
		ids = append(ids, row.ID)
	}
	if err := s.db.WithContext(ctx).Model(&db.WeComMessage{}).Where("id IN ?", ids).Update("analyzed_at", now).Error; err != nil {
		s.finishRun(ctx, &run, result, err)
		return result, err
	}
	_ = s.db.WithContext(ctx).Model(&db.WeComArchiveConfig{}).Where("id = ?", singletonConfigID).Updates(map[string]any{"last_analyze_at": now, "last_error": "", "updated_at": now}).Error
	s.finishRun(ctx, &run, result, nil)
	return result, nil
}

func (s *Service) finishRun(ctx context.Context, run *db.WeComAnalysisRun, result AnalyzeResult, runErr error) {
	now := time.Now()
	updates := map[string]any{"status": "completed", "message_count": result.MessageCount, "question_count": result.QuestionCount, "known_count": result.KnownCount, "unanswered_count": result.UnansweredCount, "completed_at": now}
	if runErr != nil {
		updates["status"] = "failed"
		updates["error"] = runErr.Error()
		s.setError(ctx, runErr)
	}
	_ = s.db.WithContext(ctx).Model(&db.WeComAnalysisRun{}).Where("id = ?", run.ID).Updates(updates).Error
}

func (s *Service) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cfg, err := s.config(ctx)
				if err != nil || !cfg.Enabled {
					continue
				}
				now := time.Now()
				if cfg.LastSyncAt == nil || now.Sub(*cfg.LastSyncAt) >= time.Duration(cfg.PollIntervalSeconds)*time.Second {
					if _, err := s.SyncNow(ctx); err != nil {
						log.Printf("[wecom] sync: %v", err)
					}
				}
				cfg, _ = s.config(ctx)
				if cfg != nil && (cfg.LastAnalyzeAt == nil || now.Sub(*cfg.LastAnalyzeAt) >= time.Duration(cfg.AnalyzeIntervalSeconds)*time.Second) {
					if _, err := s.AnalyzeDate(ctx, ""); err != nil {
						log.Printf("[wecom] analyze: %v", err)
					}
				}
			}
		}
	}()
}

func (s *Service) ListMessages(ctx context.Context, roomID, date string, page, size int) (int64, []db.WeComMessage, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	query := s.db.WithContext(ctx).Model(&db.WeComMessage{})
	if roomID != "" {
		query = query.Where("room_id = ?", roomID)
	}
	if date != "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		day, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			return 0, nil, err
		}
		query = query.Where("message_at >= ? AND message_at < ?", day, day.AddDate(0, 0, 1))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var rows []db.WeComMessage
	err := query.Order("message_at desc").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return total, rows, err
}

func (s *Service) ListRuns(ctx context.Context, limit int) ([]db.WeComAnalysisRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []db.WeComAnalysisRun
	err := s.db.WithContext(ctx).Order("started_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}
