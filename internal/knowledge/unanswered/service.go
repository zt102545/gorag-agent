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
	StatusPending   = "pending"
	StatusConverted = "converted"
	StatusIgnored   = "ignored"
)

type CaptureInput struct{ Question, SearchQuery, SessionID, RequestID, CallerUserID, ChannelID, TenantID string }
type ListQuery struct {
	Q, Status  string
	Page, Size int
}
type UpdateInput struct {
	Answer, Category string
	Tags             []string
	Status           string
}

type Service struct{ db *gorm.DB }

func NewService(gdb *gorm.DB) *Service { return &Service{db: gdb} }

func normalize(question string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(question))), " ")
}
func hash(question string) string {
	sum := sha256.Sum256([]byte(normalize(question)))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Capture(ctx context.Context, in CaptureInput) error {
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return nil
	}
	now := time.Now()
	row := db.UnansweredQuestion{ID: uuid.NewString(), Question: question, QuestionHash: hash(question), SearchQuery: strings.TrimSpace(in.SearchQuery), Status: StatusPending, Occurrences: 1, SessionID: in.SessionID, RequestID: in.RequestID, CallerUserID: in.CallerUserID, ChannelID: in.ChannelID, TenantID: in.TenantID, FirstSeenAt: now, LastSeenAt: now, UpdatedAt: now}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "question_hash"}}, DoUpdates: clause.Assignments(map[string]any{"occurrences": gorm.Expr("occurrences + 1"), "last_seen_at": now, "updated_at": now, "search_query": row.SearchQuery, "session_id": row.SessionID, "request_id": row.RequestID, "caller_user_id": row.CallerUserID, "channel_id": row.ChannelID, "tenant_id": row.TenantID})}).Create(&row).Error
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
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		query = query.Where("question LIKE ? OR answer LIKE ? OR search_query LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var rows []db.UnansweredQuestion
	err := query.Order("last_seen_at desc").Offset((q.Page - 1) * q.Size).Limit(q.Size).Find(&rows).Error
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
	updates := map[string]any{"answer": strings.TrimSpace(in.Answer), "category": strings.TrimSpace(in.Category), "tags": db.StringSlice(in.Tags), "updated_at": time.Now()}
	if in.Status != "" {
		if in.Status != StatusPending && in.Status != StatusIgnored {
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
	return s.db.WithContext(ctx).Delete(&db.UnansweredQuestion{}, "id = ?", id).Error
}
