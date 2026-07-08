package knowledgeapp

import (
	"context"

	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
)

type Service struct {
	qa *qa.Service
}

func NewService(qaService *qa.Service) *Service {
	return &Service{qa: qaService}
}

func (s *Service) Create(ctx context.Context, req qa.CreateRequest) (*qa.Item, error) {
	return s.qa.Create(ctx, req)
}

func (s *Service) Get(ctx context.Context, id string) (*qa.Item, error) {
	return s.qa.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, req qa.UpdateRequest) (*qa.Item, error) {
	return s.qa.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.qa.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, query qa.ListQuery) (*qa.ListResult, error) {
	return s.qa.List(ctx, query)
}

func (s *Service) ListCategories(ctx context.Context) ([]string, error) {
	return s.qa.ListCategories(ctx)
}

func (s *Service) ListTags(ctx context.Context) ([]string, error) {
	return s.qa.ListTags(ctx)
}

func (s *Service) EnsureIndex(ctx context.Context) error {
	return s.qa.EnsureIndex(ctx)
}
