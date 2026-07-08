package observability

import (
	"context"
	"log"

	"github.com/cloudwego/eino/callbacks"
)

func Init(_ context.Context) {
	callbacks.AppendGlobalHandlers(callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			log.Printf("[eino] start component=%s type=%s", info.Name, info.Type)
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			log.Printf("[eino] end component=%s type=%s", info.Name, info.Type)
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			log.Printf("[eino] error component=%s type=%s err=%v", info.Name, info.Type, err)
			return ctx
		}).
		Build())
}
