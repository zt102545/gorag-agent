package qa

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/db"
)

type SQLMetaStore struct {
	db *gorm.DB
}

func NewSQLMetaStore(gdb *gorm.DB) *SQLMetaStore {
	return &SQLMetaStore{db: gdb}
}

func (s *SQLMetaStore) Save(ctx context.Context, item *Item) error {
	row := toDBItem(item)
	return s.db.WithContext(ctx).Save(row).Error
}

func (s *SQLMetaStore) Get(ctx context.Context, id string) (*Item, error) {
	var row db.QAItem
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("qa not found: %s", id)
		}
		return nil, err
	}
	return fromDBItem(&row), nil
}

func (s *SQLMetaStore) Delete(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Delete(&db.QAItem{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("qa not found: %s", id)
	}
	return nil
}

func (s *SQLMetaStore) List(ctx context.Context, query ListQuery) (*ListResult, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Size <= 0 {
		query.Size = 20
	}
	q := s.db.WithContext(ctx).Model(&db.QAItem{})
	if query.KnowledgeBaseID != "" {
		q = q.Where("knowledge_base_id = ?", query.KnowledgeBaseID)
	}
	if query.Category != "" {
		q = q.Where("category = ?", query.Category)
	}
	if query.ClassifyStatus != "" {
		q = q.Where("classify_status = ?", query.ClassifyStatus)
	}
	if query.Q != "" {
		like := "%" + query.Q + "%"
		q = q.Where("question LIKE ? OR answer LIKE ?", like, like)
	}
	if query.Tag != "" {
		// JSON array text contains "tag"
		q = q.Where("tags LIKE ?", "%\""+query.Tag+"\"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []db.QAItem
	if err := q.Order("created_at desc").Offset((query.Page - 1) * query.Size).Limit(query.Size).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for i := range rows {
		items = append(items, *fromDBItem(&rows[i]))
	}
	return &ListResult{Total: total, Items: items}, nil
}

func (s *SQLMetaStore) AddToCategory(ctx context.Context, category, id string) error {
	if category == "" {
		return nil
	}
	return s.db.WithContext(ctx).Model(&db.QAItem{}).Where("id = ?", id).Update("category", category).Error
}

func (s *SQLMetaStore) RemoveFromCategory(ctx context.Context, category, id string) error {
	_ = category
	_ = id
	_ = ctx
	return nil
}

func (s *SQLMetaStore) ListByCategory(ctx context.Context, category string, page, size int) (*ListResult, error) {
	return s.List(ctx, ListQuery{Category: category, Page: page, Size: size})
}

func (s *SQLMetaStore) ListCategories(ctx context.Context, knowledgeBaseID string) ([]string, error) {
	var cats []string
	q := s.db.WithContext(ctx).Model(&db.QAItem{}).Where("category <> ''")
	if knowledgeBaseID != "" {
		q = q.Where("knowledge_base_id = ?", knowledgeBaseID)
	}
	err := q.
		Distinct("category").
		Order("category").
		Pluck("category", &cats).Error
	return cats, err
}

func (s *SQLMetaStore) ListTags(ctx context.Context, knowledgeBaseID string) ([]string, error) {
	var rows []db.QAItem
	q := s.db.WithContext(ctx).Select("tags")
	if knowledgeBaseID != "" {
		q = q.Where("knowledge_base_id = ?", knowledgeBaseID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, row := range rows {
		for _, t := range row.Tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			set[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	return out, nil
}

func (s *SQLMetaStore) FilterIDs(ctx context.Context, knowledgeBaseID string, ids []string) (map[string]struct{}, error) {
	allowed := make(map[string]struct{}, len(ids))
	if len(ids) == 0 {
		return allowed, nil
	}
	var rows []string
	if err := s.db.WithContext(ctx).Model(&db.QAItem{}).Where("knowledge_base_id = ? AND id IN ?", knowledgeBaseID, ids).Pluck("id", &rows).Error; err != nil {
		return nil, err
	}
	for _, id := range rows {
		allowed[id] = struct{}{}
	}
	return allowed, nil
}

func toDBItem(item *Item) *db.QAItem {
	status := item.ClassifyStatus
	if status == "" {
		status = ClassifyNone
	}
	return &db.QAItem{
		ID:              item.ID,
		KnowledgeBaseID: item.KnowledgeBaseID,
		Question:        item.Question,
		Answer:          item.Answer,
		Category:        item.Category,
		Tags:            db.StringSlice(item.Tags),
		ClassifyStatus:  status,
		ClassifyError:   item.ClassifyError,
		ClassifiedAt:    item.ClassifiedAt,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func fromDBItem(row *db.QAItem) *Item {
	tags := []string(row.Tags)
	if tags == nil {
		tags = []string{}
	}
	return &Item{
		ID:              row.ID,
		KnowledgeBaseID: row.KnowledgeBaseID,
		Question:        row.Question,
		Answer:          row.Answer,
		Category:        row.Category,
		Tags:            tags,
		ClassifyStatus:  row.ClassifyStatus,
		ClassifyError:   row.ClassifyError,
		ClassifiedAt:    row.ClassifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

// touchUpdated helper for classifiers
func TouchNow() time.Time { return time.Now() }
