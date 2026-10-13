package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/gorag-ai/gorag/internal/graph/rag"
)

type SearchInput struct {
	Query string `json:"query" jsonschema:"description=用户问题或检索关键词"`
}

type searchObserverKey struct{}

// WithSearchObserver records the result of the Agent's actual tool call for
// response metadata and tracing without issuing a second retrieval.
func WithSearchObserver(ctx context.Context, observer func(string, []rag.SearchHit)) context.Context {
	if observer == nil {
		return ctx
	}
	return context.WithValue(ctx, searchObserverKey{}, observer)
}

func NewRAGSearchTool(service *rag.Service) (tool.InvokableTool, error) {
	return utils.InferTool("knowledge_search", "检索 QA 知识库并返回多条候选问答（含标准答案）；请自行比较相关性后组织最终回复", func(ctx context.Context, input *SearchInput) (string, error) {
		if input.Query == "" {
			return "", fmt.Errorf("query is required")
		}
		hits, err := service.SearchHits(ctx, input.Query)
		if err != nil {
			return "", err
		}
		if observer, ok := ctx.Value(searchObserverKey{}).(func(string, []rag.SearchHit)); ok {
			observer(input.Query, hits)
		}
		if len(hits) == 0 {
			return "未找到相关 QA 知识库内容。", nil
		}
		return rag.FormatHits(hits), nil
	})
}

type TransferInput struct {
	Reason  string `json:"reason" jsonschema:"description=转人工原因"`
	Summary string `json:"summary" jsonschema:"description=当前对话摘要"`
}

func NewTransferHumanTool(resolveHandoffReply func(context.Context) string) (tool.InvokableTool, error) {
	return utils.InferTool("transfer_human", "将用户转接给人工客服", func(ctx context.Context, input *TransferInput) (string, error) {
		reply := ""
		if resolveHandoffReply != nil {
			reply = strings.TrimSpace(resolveHandoffReply(ctx))
		}
		if reply == "" {
			return fmt.Sprintf("已为你记录人工客服诉求，请稍候。原因：%s", input.Reason), nil
		}
		return reply, nil
	})
}
