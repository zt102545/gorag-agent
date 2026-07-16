package knowledgeapp

import (
	"context"

	"github.com/gorag-ai/gorag/internal/knowledge/qa"
)

type ClassificationJobService struct {
	qa *qa.Service
}

func NewClassificationJobService(qaService *qa.Service) *ClassificationJobService {
	return &ClassificationJobService{qa: qaService}
}

func (s *ClassificationJobService) Reclassify(ctx context.Context, id string) (*qa.Item, error) {
	return s.qa.Reclassify(ctx, id)
}

func (s *ClassificationJobService) Enqueue(ctx context.Context, id string) error {
	return s.qa.EnqueueClassify(ctx, id)
}
