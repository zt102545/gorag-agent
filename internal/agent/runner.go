package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
)

type Runner struct {
	inner *adk.Runner
}

func NewRunner(ctx context.Context, agent adk.Agent, store adk.CheckPointStore) *Runner {
	return &Runner{
		inner: adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
			CheckPointStore: store,
		}),
	}
}

func (r *Runner) Inner() *adk.Runner {
	return r.inner
}
