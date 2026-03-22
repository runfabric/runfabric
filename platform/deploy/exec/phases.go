package exec

import "context"

type Phase interface {
	Name() string
	Run(ctx context.Context, execCtx *Context) error
}

type PhaseFunc struct {
	PhaseName string
	Fn        func(ctx context.Context, execCtx *Context) error
}

func (p PhaseFunc) Name() string {
	return p.PhaseName
}

func (p PhaseFunc) Run(ctx context.Context, execCtx *Context) error {
	return p.Fn(ctx, execCtx)
}
