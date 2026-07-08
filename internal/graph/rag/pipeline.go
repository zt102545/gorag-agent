package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/retriever"
)

type Service struct {
	retriever retriever.Retriever
}

func NewService(components *Components) *Service {
	return &Service{retriever: components.Retriever}
}

func (s *Service) Search(ctx context.Context, query string) (string, error) {
	docs, err := s.retriever.Retrieve(ctx, query)
	if err != nil {
		return "", fmt.Errorf("retrieve documents: %w", err)
	}
	if len(docs) == 0 {
		return "未找到相关 QA 知识库内容。", nil
	}

	var builder strings.Builder
	for i, doc := range docs {
		answer, _ := doc.MetaData["answer"].(string)
		category, _ := doc.MetaData["category"].(string)

		builder.WriteString(fmt.Sprintf("[%d]", i+1))
		if category != "" {
			builder.WriteString(fmt.Sprintf(" 分类: %s", category))
		}
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Q: %s\n", doc.Content))
		builder.WriteString(fmt.Sprintf("A: %s\n", answer))
		if i < len(docs)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

func (s *Service) BuildPrompt(query, context string) string {
	return strings.NewReplacer(
		"{{context}}", context,
		"{{query}}", query,
	).Replace(defaultPrompt)
}
