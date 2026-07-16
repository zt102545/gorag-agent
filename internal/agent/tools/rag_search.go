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
	Query string `json:"query" jsonschema:"description=鐢ㄦ埛闂鎴栨绱㈠叧閿瘝"`
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
	return utils.InferTool("knowledge_search", "妫€绱?QA 鐭ヨ瘑搴撳苟杩斿洖澶氭潯鍊欓€夐棶绛旓紙鍚爣鍑嗙瓟妗堬級锛涜鑷姣旇緝鐩稿叧鎬у悗缁勭粐鏈€缁堝洖澶?, func(ctx context.Context, input *SearchInput) (string, error) {
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
			return "鏈壘鍒扮浉鍏?QA 鐭ヨ瘑搴撳唴瀹广€?, nil
		}
		return rag.FormatHits(hits), nil
	})
}

type TransferInput struct {
	Reason  string `json:"reason" jsonschema:"description=杞汉宸ュ師鍥?`
	Summary string `json:"summary" jsonschema:"description=褰撳墠瀵硅瘽鎽樿"`
}

func NewTransferHumanTool(resolveHandoffReply func(context.Context) string) (tool.InvokableTool, error) {
	return utils.InferTool("transfer_human", "灏嗙敤鎴疯浆鎺ョ粰浜哄伐瀹㈡湇", func(ctx context.Context, input *TransferInput) (string, error) {
		reply := ""
		if resolveHandoffReply != nil {
			reply = strings.TrimSpace(resolveHandoffReply(ctx))
		}
		if reply == "" {
			return fmt.Sprintf("宸蹭负浣犺褰曚汉宸ュ鏈嶈瘔姹傦紝璇风◢鍊欍€傚師鍥狅細%s", input.Reason), nil
		}
		return reply, nil
	})
}
