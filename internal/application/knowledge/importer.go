package knowledgeapp

import (
	"context"
	"io"

	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
)

func (s *Service) ImportCSV(ctx context.Context, r io.Reader) (*qa.ImportResult, error) {
	return s.qa.ImportCSV(ctx, r)
}
