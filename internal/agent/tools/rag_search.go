package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/yourorg/agent_customer_service/internal/graph/rag"
)

type SearchInput struct {
	Query string `json:"query" jsonschema:"description=用户问题或检索关键词"`
}

func NewRAGSearchTool(service *rag.Service) (tool.InvokableTool, error) {
	return utils.InferTool("knowledge_search", "检索 QA 知识库，返回与用户问题相似的问答对（含标准答案）", func(ctx context.Context, input *SearchInput) (string, error) {
		if input.Query == "" {
			return "", fmt.Errorf("query is required")
		}
		return service.Search(ctx, input.Query)
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
