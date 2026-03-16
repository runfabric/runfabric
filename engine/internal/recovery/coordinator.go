package recovery

import "context"

type Handler interface {
	Rollback(ctx context.Context, req Request) (*Result, error)
	Resume(ctx context.Context, req Request) (*Result, error)
	Inspect(ctx context.Context, req Request) (*Result, error)
}
