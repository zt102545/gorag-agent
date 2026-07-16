package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/gorag-ai/gorag/internal/db"
)

type AIErrorService struct {
	db *gorm.DB
}

type AIErrorInput struct {
	Source    string `json:"source"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Details   string `json:"details,omitempty"`
}

func NewAIErrorService(gdb *gorm.DB) *AIErrorService {
	return &AIErrorService{db: gdb}
}

func (s *AIErrorService) Report(ctx context.Context, input AIErrorInput) error {
	row := &db.AIErrorLog{
		ID:        uuid.NewString(),
		Source:    input.Source,
		Stage:     input.Stage,
		Message:   input.Message,
		RequestID: input.RequestID,
		SessionID: input.SessionID,
		Details:   input.Details,
		Resolved:  false,
		CreatedAt: time.Now(),
	}
	return s.db.WithContext(ctx).Create(row).Error
}

func (s *AIErrorService) List(ctx context.Context) ([]db.AIErrorLog, error) {
	var rows []db.AIErrorLog
	err := s.db.WithContext(ctx).Order("created_at desc").Limit(200).Find(&rows).Error
	return rows, err
}

func (s *AIErrorService) UnresolvedCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&db.AIErrorLog{}).Where("resolved = ?", false).Count(&count).Error
	return count, err
}

func (s *AIErrorService) Resolve(ctx context.Context, id string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&db.AIErrorLog{}).Where("id = ?", id).Updates(map[string]any{
		"resolved":    true,
		"resolved_at": &now,
	}).Error
}
