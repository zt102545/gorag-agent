package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	agenttools "github.com/yourorg/agent_customer_service/internal/agent/tools"
	"github.com/yourorg/agent_customer_service/internal/graph/rag"
)

func NewCustomerServiceAgent(ctx context.Context, chatModel model.ToolCallingChatModel, ragService *rag.Service, instruction string) (adk.Agent, error) {
	if instruction == "" {
		instruction = CustomerServiceInstruction
	}
	ragTool, err := agenttools.NewRAGSearchTool(ragService)
	if err != nil {
		return nil, fmt.Errorf("create rag tool: %w", err)
	}
	transferTool, err := agenttools.NewTransferHumanTool()
	if err != nil {
		return nil, fmt.Errorf("create transfer tool: %w", err)
	}

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "CustomerServiceAgent",
		Description: "智能客服助手，可检索知识库并在必要时转人工",
		Instruction: instruction,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{ragTool, transferTool},
			},
		},
	})
}
