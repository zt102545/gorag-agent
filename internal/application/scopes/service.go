package scopes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/gorag-ai/gorag/internal/appscope"
	"github.com/gorag-ai/gorag/internal/db"
	"github.com/gorag-ai/gorag/internal/knowledge/qa"
)

type Service struct {
	db *gorm.DB
	qa *qa.Service
}

type UpsertRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type CopyRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func SeedDefaults(ctx context.Context, gdb *gorm.DB) error {
	now := time.Now()
	return gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		promptGroup := db.PromptGroup{ID: appscope.DefaultPromptGroupID, Name: "默认提示词组", Description: "升级前已有提示词", IsDefault: true, CreatedAt: now, UpdatedAt: now}
		if err := tx.Where("id = ?", promptGroup.ID).FirstOrCreate(&promptGroup).Error; err != nil {
			return err
		}
		var prompts []db.Prompt
		if err := tx.Find(&prompts).Error; err != nil {
			return err
		}
		for _, prompt := range prompts {
			var count int64
			if err := tx.Model(&db.PromptGroupItem{}).Where("group_id = ? AND prompt_key = ?", promptGroup.ID, prompt.Key).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				item := db.PromptGroupItem{ID: uuid.NewString(), GroupID: promptGroup.ID, Key: prompt.Key, Content: prompt.Content, UpdatedAt: prompt.UpdatedAt}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
		}
		base := db.KnowledgeBase{ID: appscope.DefaultKnowledgeBaseID, Name: "默认知识库", Description: "升级前已有 QA", IsDefault: true, CreatedAt: now, UpdatedAt: now}
		if err := tx.Where("id = ?", base.ID).FirstOrCreate(&base).Error; err != nil {
			return err
		}
		return tx.Model(&db.QAItem{}).Where("knowledge_base_id = '' OR knowledge_base_id IS NULL").Update("knowledge_base_id", base.ID).Error
	})
}

func NewService(gdb *gorm.DB, qaService *qa.Service) *Service {
	return &Service{db: gdb, qa: qaService}
}

func (s *Service) ListPromptGroups(ctx context.Context) ([]db.PromptGroup, error) {
	var rows []db.PromptGroup
	err := s.db.WithContext(ctx).Order("is_default desc, created_at asc").Find(&rows).Error
	return rows, err
}

func (s *Service) CreatePromptGroup(ctx context.Context, req UpsertRequest) (*db.PromptGroup, error) {
	row := &db.PromptGroup{ID: uuid.NewString(), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if row.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		var items []db.PromptGroupItem
		if err := tx.Where("group_id = ?", appscope.DefaultPromptGroupID).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			item.ID, item.GroupID, item.UpdatedAt = uuid.NewString(), row.ID, time.Now()
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdatePromptGroup(ctx context.Context, id string, req UpsertRequest) (*db.PromptGroup, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := s.db.WithContext(ctx).Model(&db.PromptGroup{}).Where("id = ?", id).Updates(map[string]any{"name": strings.TrimSpace(req.Name), "description": strings.TrimSpace(req.Description), "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	var row db.PromptGroup
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) CopyPromptGroup(ctx context.Context, sourceID string, req CopyRequest) (*db.PromptGroup, error) {
	var result *db.PromptGroup
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source db.PromptGroup
		if err := tx.First(&source, "id = ?", sourceID).Error; err != nil {
			return err
		}
		row := &db.PromptGroup{ID: uuid.NewString(), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if row.Name == "" {
			return fmt.Errorf("name is required")
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		var items []db.PromptGroupItem
		if err := tx.Where("group_id = ?", sourceID).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			item.ID, item.GroupID, item.UpdatedAt = uuid.NewString(), row.ID, time.Now()
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		result = row
		return nil
	})
	return result, err
}

func (s *Service) DeletePromptGroup(ctx context.Context, id string) error {
	if id == appscope.DefaultPromptGroupID {
		return fmt.Errorf("default prompt group cannot be deleted")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&db.CallerToken{}).Where("prompt_group_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("prompt group is bound to caller applications")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&db.PromptGroupItem{}, "group_id = ?", id).Error; err != nil {
			return err
		}
		res := tx.Delete(&db.PromptGroup{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *Service) ListPrompts(ctx context.Context, groupID string) ([]db.PromptGroupItem, error) {
	if groupID == "" {
		groupID = appscope.DefaultPromptGroupID
	}
	var rows []db.PromptGroupItem
	err := s.db.WithContext(ctx).Where("group_id = ?", groupID).Order("prompt_key").Find(&rows).Error
	return rows, err
}

func (s *Service) UpdatePrompt(ctx context.Context, groupID, key, content string) (*db.PromptGroupItem, error) {
	if groupID == "" {
		groupID = appscope.DefaultPromptGroupID
	}
	var group db.PromptGroup
	if err := s.db.WithContext(ctx).First(&group, "id = ?", groupID).Error; err != nil {
		return nil, err
	}
	var item db.PromptGroupItem
	err := s.db.WithContext(ctx).Where("group_id = ? AND prompt_key = ?", groupID, key).First(&item).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		item = db.PromptGroupItem{ID: uuid.NewString(), GroupID: groupID, Key: key}
	}
	item.Content, item.UpdatedAt = content, time.Now()
	if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) ListKnowledgeBases(ctx context.Context) ([]db.KnowledgeBase, error) {
	var rows []db.KnowledgeBase
	err := s.db.WithContext(ctx).Order("is_default desc, created_at asc").Find(&rows).Error
	return rows, err
}

func (s *Service) CreateKnowledgeBase(ctx context.Context, req UpsertRequest) (*db.KnowledgeBase, error) {
	row := &db.KnowledgeBase{ID: uuid.NewString(), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if row.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdateKnowledgeBase(ctx context.Context, id string, req UpsertRequest) (*db.KnowledgeBase, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := s.db.WithContext(ctx).Model(&db.KnowledgeBase{}).Where("id = ?", id).Updates(map[string]any{"name": strings.TrimSpace(req.Name), "description": strings.TrimSpace(req.Description), "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	var row db.KnowledgeBase
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) CopyKnowledgeBase(ctx context.Context, sourceID string, req CopyRequest) (*db.KnowledgeBase, error) {
	row, err := s.CreateKnowledgeBase(ctx, UpsertRequest(req))
	if err != nil {
		return nil, err
	}
	page := 1
	for {
		result, listErr := s.qa.List(ctx, qa.ListQuery{KnowledgeBaseID: sourceID, Page: page, Size: 100})
		if listErr != nil {
			_ = s.cleanupKnowledgeBase(ctx, row.ID)
			return nil, listErr
		}
		for _, item := range result.Items {
			_, createErr := s.qa.Create(ctx, qa.CreateRequest{KnowledgeBaseID: row.ID, Question: item.Question, Answer: item.Answer, Category: item.Category, Tags: item.Tags})
			if createErr != nil {
				_ = s.cleanupKnowledgeBase(ctx, row.ID)
				return nil, createErr
			}
		}
		if page*100 >= int(result.Total) {
			break
		}
		page++
	}
	return row, nil
}

func (s *Service) DeleteKnowledgeBase(ctx context.Context, id string) error {
	if id == appscope.DefaultKnowledgeBaseID {
		return fmt.Errorf("default knowledge base cannot be deleted")
	}
	var bindings, items int64
	if err := s.db.WithContext(ctx).Model(&db.CallerToken{}).Where("knowledge_base_id = ?", id).Count(&bindings).Error; err != nil {
		return err
	}
	if bindings > 0 {
		return fmt.Errorf("knowledge base is bound to caller applications")
	}
	if err := s.db.WithContext(ctx).Model(&db.QAItem{}).Where("knowledge_base_id = ?", id).Count(&items).Error; err != nil {
		return err
	}
	if items > 0 {
		return fmt.Errorf("knowledge base is not empty")
	}
	res := s.db.WithContext(ctx).Delete(&db.KnowledgeBase{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) cleanupKnowledgeBase(ctx context.Context, id string) error {
	result, _ := s.qa.List(ctx, qa.ListQuery{KnowledgeBaseID: id, Page: 1, Size: 10000})
	if result != nil {
		for _, item := range result.Items {
			_ = s.qa.Delete(ctx, item.ID)
		}
	}
	return s.db.WithContext(ctx).Delete(&db.KnowledgeBase{}, "id = ?", id).Error
}
