package unanswered

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/yourorg/agent_customer_service/internal/db"
)

const (
	StatusPending         = "pending"
	StatusConverted       = "converted"
	StatusIgnored         = "ignored"
	ReasonNoHit           = "no_hit"
	ReasonLowConfidence   = "low_confidence"
	ReasonEmptyQAAnswer   = "empty_qa_answer"
	ReasonModelEmpty      = "model_empty"
	ReasonModelUnanswered = "model_unanswered"
)

type CaptureInput struct {
	Question, SearchQuery, Reason, Intent, FinalAnswer      string
	SessionID, RequestID, CallerUserID, ChannelID, TenantID string
	IntentConfidence, TopScore                              float64
	HitCount                                                int
}
type ListQuery struct {
	Q, Status, Reason, TenantID, ChannelID, Sort string
	Page, Size                                   int
}
type UpdateInput struct {
	Answer, Category string
	Tags             []string
	Status           string
}

type Service struct{ db *gorm.DB }

type Stats struct {
	Total       int64            `json:"total"`
	Pending     int64            `json:"pending"`
	Converted   int64            `json:"converted"`
	Ignored     int64            `json:"ignored"`
	Events7Days int64            `json:"events_7_days"`
	ByReason    map[string]int64 `json:"by_reason"`
}

func NewService(gdb *gorm.DB) *Service { return &Service{db: gdb} }

func normalize(question string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(question))), " ")
}
func hash(tenantID, question string) string {
	// A tenant-scoped hash prevents one customer's workflow and context from
	// overwriting another customer's identical question.
	sum := sha256.Sum256([]byte(strings.TrimSpace(tenantID) + "\x00" + normalize(question)))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Capture(ctx context.Context, in CaptureInput) error {
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return nil
	}
	now := time.Now()
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = ReasonNoHit
	}
	row := db.UnansweredQuestion{ID: uuid.NewString(), Question: question, QuestionHash: hash(in.TenantID, question), SearchQuery: strings.TrimSpace(in.SearchQuery), Reason: reason, Status: StatusPending, Occurrences: 1, SessionID: in.SessionID, RequestID: in.RequestID, CallerUserID: in.CallerUserID, ChannelID: in.ChannelID, TenantID: in.TenantID, FirstSeenAt: now, LastSeenAt: now, UpdatedAt: now}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"occurrences": gorm.Expr("occurrences + 1"), "last_seen_at": now, "updated_at": now,
			"search_query": row.SearchQuery, "reason": reason, "session_id": row.SessionID,
			"request_id": row.RequestID, "caller_user_id": row.CallerUserID,
			"channel_id": row.ChannelID, "tenant_id": row.TenantID,
			"status": gorm.Expr("CASE WHEN status = ? THEN ? ELSE status END", StatusConverted, StatusPending),
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "question_hash"}}, DoUpdates: clause.Assignments(updates)}).Create(&row).Error; err != nil {
			return err
		}
		var aggregate db.UnansweredQuestion
		if err := tx.First(&aggregate, "question_hash = ?", row.QuestionHash).Error; err != nil {
			return err
		}
		event := db.UnansweredQuestionEvent{ID: uuid.NewString(), QuestionID: aggregate.ID, Question: question, SearchQuery: strings.TrimSpace(in.SearchQuery), Reason: reason, Intent: strings.TrimSpace(in.Intent), IntentConfidence: in.IntentConfidence, HitCount: in.HitCount, TopScore: in.TopScore, FinalAnswer: strings.TrimSpace(in.FinalAnswer), SessionID: in.SessionID, RequestID: in.RequestID, CallerUserID: in.CallerUserID, ChannelID: in.ChannelID, TenantID: in.TenantID, CreatedAt: now}
		return tx.Create(&event).Error
	})
}

func (s *Service) List(ctx context.Context, q ListQuery) (int64, []db.UnansweredQuestion, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}
	query := s.db.WithContext(ctx).Model(&db.UnansweredQuestion{})
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Reason != "" {
		query = query.Where("reason = ?", q.Reason)
	}
	if q.TenantID != "" {
		query = query.Where("tenant_id = ?", q.TenantID)
	}
	if q.ChannelID != "" {
		query = query.Where("channel_id = ?", q.ChannelID)
	}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		query = query.Where("question LIKE ? OR answer LIKE ? OR search_query LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var rows []db.UnansweredQuestion
	order := "last_seen_at desc"
	if q.Sort == "occurrences" {
		order = "occurrences desc, last_seen_at desc"
	}
	err := query.Order(order).Offset((q.Page - 1) * q.Size).Limit(q.Size).Find(&rows).Error
	return total, rows, err
}

func (s *Service) Get(ctx context.Context, id string) (*db.UnansweredQuestion, error) {
	var row db.UnansweredQuestion
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*db.UnansweredQuestion, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status == StatusConverted && in.Status != "" && in.Status != StatusConverted {
		return nil, fmt.Errorf("converted question cannot be reopened manually")
	}
	updates := map[string]any{"answer": strings.TrimSpace(in.Answer), "category": strings.TrimSpace(in.Category), "tags": db.StringSlice(in.Tags), "updated_at": time.Now()}
	if in.Status != "" {
		if in.Status != StatusPending && in.Status != StatusIgnored && !(current.Status == StatusConverted && in.Status == StatusConverted) {
			return nil, fmt.Errorf("invalid status")
		}
		updates["status"] = in.Status
	}
	if err := s.db.WithContext(ctx).Model(&db.UnansweredQuestion{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}
func (s *Service) MarkConverted(ctx context.Context, id, qaID string) error {
	return s.db.WithContext(ctx).Model(&db.UnansweredQuestion{}).Where("id = ?", id).Updates(map[string]any{"status": StatusConverted, "converted_qa_id": qaID, "updated_at": time.Now()}).Error
}
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&db.UnansweredQuestionEvent{}, "question_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&db.UnansweredQuestion{}, "id = ?", id).Error
	})
}

func (s *Service) ListEvents(ctx context.Context, id string, limit int) ([]db.UnansweredQuestionEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var rows []db.UnansweredQuestionEvent
	err := s.db.WithContext(ctx).Where("question_id = ?", id).Order("created_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func (s *Service) Stats(ctx context.Context) (Stats, error) {
	stats := Stats{ByReason: map[string]int64{}}
	base := s.db.WithContext(ctx).Model(&db.UnansweredQuestion{})
	if err := base.Count(&stats.Total).Error; err != nil {
		return stats, err
	}
	for status, target := range map[string]*int64{StatusPending: &stats.Pending, StatusConverted: &stats.Converted, StatusIgnored: &stats.Ignored} {
		if err := s.db.WithContext(ctx).Model(&db.UnansweredQuestion{}).Where("status = ?", status).Count(target).Error; err != nil {
			return stats, err
		}
	}
	type reasonCount struct {
		Reason string
		Count  int64
	}
	var reasons []reasonCount
	if err := s.db.WithContext(ctx).Model(&db.UnansweredQuestion{}).Select("reason, count(*) as count").Group("reason").Scan(&reasons).Error; err != nil {
		return stats, err
	}
	for _, row := range reasons {
		stats.ByReason[row.Reason] = row.Count
	}
	if err := s.db.WithContext(ctx).Model(&db.UnansweredQuestionEvent{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&stats.Events7Days).Error; err != nil {
		return stats, err
	}
	return stats, nil
}
