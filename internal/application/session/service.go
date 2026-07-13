package sessionapp

import (
	"context"

	"github.com/cloudwego/eino/schema"

	sessiondomain "github.com/yourorg/agent_customer_service/internal/session"
)

type Store interface {
	sessiondomain.Store
	ListWithStats(ctx context.Context, query sessiondomain.ListQuery) (int64, []sessiondomain.SessionListItem, error)
	GetMessageViews(ctx context.Context, sessionID string) ([]sessiondomain.MessageView, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, params sessiondomain.CreateParams) (*sessiondomain.Meta, error) {
	return s.store.Create(ctx, params)
}

func (s *Service) Get(ctx context.Context, sessionID string) (*sessiondomain.Meta, error) {
	return s.store.Get(ctx, sessionID)
}

func (s *Service) Delete(ctx context.Context, sessionID string) error {
	return s.store.Delete(ctx, sessionID)
}

func (s *Service) ClearHistory(ctx context.Context, sessionID string) error {
	return s.store.ClearMessages(ctx, sessionID)
}

func (s *Service) History(ctx context.Context, sessionID string) ([]*sessiondomain.MessageView, error) {
	views, err := s.store.GetMessageViews(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]*sessiondomain.MessageView, 0, len(views))
	for i := range views {
		view := views[i]
		out = append(out, &view)
	}
	return out, nil
}

func (s *Service) RawHistory(ctx context.Context, sessionID string) ([]*schema.Message, error) {
	return s.store.GetMessages(ctx, sessionID)
}

func (s *Service) ListByUser(ctx context.Context, query sessiondomain.ListQuery) (int64, []sessiondomain.SessionListItem, error) {
	return s.store.ListWithStats(ctx, query)
}

func (s *Service) AdminList(ctx context.Context, query sessiondomain.ListQuery) (int64, []sessiondomain.SessionListItem, error) {
	return s.store.ListWithStats(ctx, query)
}

func (s *Service) AdminGet(ctx context.Context, sessionID string) (*sessiondomain.Meta, []sessiondomain.MessageView, error) {
	meta, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	messages, err := s.store.GetMessageViews(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return meta, messages, nil
}
