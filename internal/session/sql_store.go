package session

import (
	"context"
	"encoding/json"
	"fmt"
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

func (s *SQLStore) Create(ctx context.Context) (*Meta, error) {
	now := time.Now()
	row := &db.Session{
		ID:        uuid.NewString(),
		Status:    "active",
		Meta:      db.JSONMap{},
		CreatedAt: now,
		UpdatedAt: now,
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

func (s *SQLStore) AppendMessages(ctx context.Context, sessionID string, messages ...*schema.Message) error {
	if len(messages) == 0 {
		return nil
	}
	if _, err := s.Get(ctx, sessionID); err != nil {
		return err
	}
	now := time.Now()
	rows := make([]db.Message, 0, len(messages))
	for _, msg := range messages {
		extra := ""
		if msg != nil && msg.ToolCalls != nil {
			if b, err := json.Marshal(msg.ToolCalls); err == nil {
				extra = string(b)
			}
		}
		content := ""
		role := "user"
		if msg != nil {
			content = msg.Content
			role = string(msg.Role)
		}
		rows = append(rows, db.Message{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			Role:      role,
			Content:   content,
			Extra:     extra,
			CreatedAt: now,
		})
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
		return tx.Model(&db.Session{}).Where("id = ?", sessionID).Update("updated_at", now).Error
	})
}

func (s *SQLStore) List(ctx context.Context, page, size int, status string) (int64, []Meta, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	q := s.db.WithContext(ctx).Model(&db.Session{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var rows []db.Session
	if err := q.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return 0, nil, err
	}
	items := make([]Meta, 0, len(rows))
	for i := range rows {
		m := toMeta(&rows[i])
		items = append(items, *m)
	}
	return total, items, nil
}

func toMeta(row *db.Session) *Meta {
	return &Meta{
		ID:        row.ID,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		ClosedAt:  row.ClosedAt,
	}
}
