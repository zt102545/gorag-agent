package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/db"
)

type SQLStore struct {
	db *gorm.DB
}

func NewSQLStore(gdb *gorm.DB) *SQLStore {
	return &SQLStore{db: gdb}
}

func (s *SQLStore) Create(ctx context.Context, params CreateParams) (*Meta, error) {
	now := time.Now()
	row := &db.Session{
		ID:             uuid.NewString(),
		Title:          "",
		ExternalUserID: params.ExternalUserID,
		ChannelID:      params.ChannelID,
		TenantID:       params.TenantID,
		CallerTokenID:  params.CallerTokenID,
		Status:         "active",
		Meta:           db.JSONMap(params.Meta),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if row.Meta == nil {
		row.Meta = db.JSONMap{}
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return toMeta(row), nil
}

func (s *SQLStore) Get(ctx context.Context, sessionID string) (*Meta, error) {
	var row db.Session
	if err := s.db.WithContext(ctx).First(&row, "id = ?", sessionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, err
	}
	return toMeta(&row), nil
}

func (s *SQLStore) Delete(ctx context.Context, sessionID string) error {
	now := time.Now()
	res := s.db.WithContext(ctx).Model(&db.Session{}).Where("id = ?", sessionID).Updates(map[string]any{
		"status":     "closed",
		"closed_at":  now,
		"updated_at": now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return nil
}

func (s *SQLStore) GetMessages(ctx context.Context, sessionID string) ([]*schema.Message, error) {
	var rows []db.Message
	if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*schema.Message, 0, len(rows))
	for _, row := range rows {
		msg := &schema.Message{
			Role:    schema.RoleType(row.Role),
			Content: row.Content,
		}
		out = append(out, msg)
	}
	return out, nil
}

func (s *SQLStore) GetMessageViews(ctx context.Context, sessionID string) ([]MessageView, error) {
	var rows []db.Message
	if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]MessageView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMessageView(row))
	}
	return out, nil
}

func (s *SQLStore) AppendMessages(ctx context.Context, sessionID string, params AppendParams, messages ...*schema.Message) error {
	if len(messages) == 0 {
		return nil
	}
	if _, err := s.Get(ctx, sessionID); err != nil {
		return err
	}
	now := time.Now()
	rows := make([]db.Message, 0, len(messages))
	for i, msg := range messages {
		createdAt := now.Add(time.Duration(i) * time.Nanosecond)
		extraData := map[string]any{}
		for k, v := range params.Extra {
			extraData[k] = v
		}
		if msg != nil && msg.ToolCalls != nil {
			extraData["tool_calls"] = msg.ToolCalls
		}
		extra := ""
		if len(extraData) > 0 {
			if b, err := json.Marshal(extraData); err == nil {
				extra = string(b)
			}
		}
		content := ""
		role := "user"
		if msg != nil {
			content = msg.Content
			role = string(msg.Role)
		}
		clientMessageID := ""
		if role == string(schema.User) {
			clientMessageID = params.ClientMessageID
		}
		rows = append(rows, db.Message{
			ID:              uuid.NewString(),
			SessionID:       sessionID,
			Role:            role,
			RequestID:       params.RequestID,
			ClientMessageID: clientMessageID,
			Content:         content,
			Extra:           extra,
			CreatedAt:       createdAt,
		})
	}
	if params.ClientMessageID != "" {
		var count int64
		if err := s.db.WithContext(ctx).Model(&db.Message{}).
			Where("session_id = ? AND client_message_id = ?", sessionID, params.ClientMessageID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"updated_at": now,
		}
		if title := firstUserTitle(messages); title != "" {
			updates["title"] = gorm.Expr("CASE WHEN title IS NULL OR title = '' THEN ? ELSE title END", title)
		}
		return tx.Model(&db.Session{}).Where("id = ?", sessionID).Updates(updates).Error
	})
}

func (s *SQLStore) List(ctx context.Context, query ListQuery) (int64, []Meta, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Size <= 0 {
		query.Size = 20
	}
	q := s.db.WithContext(ctx).Model(&db.Session{})
	if query.SessionID != "" {
		q = q.Where("id = ?", query.SessionID)
	}
	if query.Q != "" {
		like := "%" + strings.TrimSpace(query.Q) + "%"
		q = q.Where("id LIKE ? OR title LIKE ? OR external_user_id LIKE ? OR channel_id LIKE ? OR tenant_id LIKE ? OR EXISTS (SELECT 1 FROM messages WHERE messages.session_id = sessions.id AND messages.content LIKE ?)",
			like, like, like, like, like, like)
	}
	if query.Status != "" {
		q = q.Where("status = ?", query.Status)
	}
	if query.ExternalUserID != "" {
		q = q.Where("external_user_id = ?", query.ExternalUserID)
	}
	if query.ChannelID != "" {
		q = q.Where("channel_id = ?", query.ChannelID)
	}
	if query.TenantID != "" {
		q = q.Where("tenant_id = ?", query.TenantID)
	}
	if query.CallerTokenID != "" {
		q = q.Where("caller_token_id = ?", query.CallerTokenID)
	}
	if query.CreatedFrom != nil {
		q = q.Where("created_at >= ?", *query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		q = q.Where("created_at <= ?", *query.CreatedTo)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var rows []db.Session
	if err := q.Order("created_at desc").Offset((query.Page - 1) * query.Size).Limit(query.Size).Find(&rows).Error; err != nil {
		return 0, nil, err
	}
	items := make([]Meta, 0, len(rows))
	for i := range rows {
		m := toMeta(&rows[i])
		items = append(items, *m)
	}
	return total, items, nil
}

func (s *SQLStore) ListWithStats(ctx context.Context, query ListQuery) (int64, []SessionListItem, error) {
	total, metas, err := s.List(ctx, query)
	if err != nil {
		return 0, nil, err
	}
	items := make([]SessionListItem, 0, len(metas))
	for _, meta := range metas {
		item := SessionListItem{Meta: meta}
		var count int64
		_ = s.db.WithContext(ctx).Model(&db.Message{}).Where("session_id = ?", meta.ID).Count(&count).Error
		item.MessageCount = count

		var last db.Message
		if err := s.db.WithContext(ctx).Where("session_id = ?", meta.ID).Order("created_at desc").First(&last).Error; err == nil {
			item.LastMessageRole = last.Role
			item.LastMessagePreview = trimPreview(last.Content)
			item.LastMessageAt = &last.CreatedAt
		}
		items = append(items, item)
	}
	return total, items, nil
}

func toMeta(row *db.Session) *Meta {
	return &Meta{
		ID:             row.ID,
		Title:          row.Title,
		ExternalUserID: row.ExternalUserID,
		ChannelID:      row.ChannelID,
		TenantID:       row.TenantID,
		CallerTokenID:  row.CallerTokenID,
		Status:         row.Status,
		Meta:           map[string]any(row.Meta),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		ClosedAt:       row.ClosedAt,
	}
}

func toMessageView(row db.Message) MessageView {
	view := MessageView{
		ID:              row.ID,
		Role:            row.Role,
		Content:         row.Content,
		RequestID:       row.RequestID,
		ClientMessageID: row.ClientMessageID,
		CreatedAt:       row.CreatedAt,
	}
	if strings.TrimSpace(row.Extra) != "" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(row.Extra), &extra); err == nil {
			view.Extra = extra
		} else {
			var raw any
			if err := json.Unmarshal([]byte(row.Extra), &raw); err == nil {
				view.Extra = map[string]any{"raw": raw}
			} else {
				view.Extra = map[string]any{"raw": row.Extra}
			}
		}
	}
	return view
}

func trimPreview(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= 80 {
		return content
	}
	return content[:80] + "..."
}

func firstUserTitle(messages []*schema.Message) string {
	for _, msg := range messages {
		if msg == nil || msg.Role != schema.User {
			continue
		}
		title := strings.TrimSpace(msg.Content)
		if title == "" {
			continue
		}
		title = strings.ReplaceAll(title, "\r", " ")
		title = strings.ReplaceAll(title, "\n", " ")
		if len(title) <= 40 {
			return title
		}
		return title[:40] + "..."
	}
	return ""
}
