package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

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

func NewTransferHumanTool() (tool.InvokableTool, error) {
	return utils.InferTool("transfer_human", "将用户转接给人工客服", func(ctx context.Context, input *TransferInput) (string, error) {
		if isResume, hasData, approved := compose.GetResumeContext[bool](ctx); isResume && hasData {
			if approved {
				return fmt.Sprintf("已转接人工客服。原因：%s", input.Reason), nil
			}
			return "用户取消了转人工请求，将继续由智能客服提供服务。", nil
		}

		message := fmt.Sprintf("用户请求转人工。原因：%s；摘要：%s。是否批准转接？", input.Reason, input.Summary)
		return "", compose.Interrupt(ctx, message)
	})
}
