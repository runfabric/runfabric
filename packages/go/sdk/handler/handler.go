// Package handler provides the RunFabric handler contract for Go: (event, context) -> response.
package handler

import "context"

// Context holds request metadata (stage, function name, request ID).
type Context struct {
	Stage        string
	FunctionName string
	RequestID    string
}

// Handler is the RunFabric function signature: event (JSON-like map) and context -> response (map or error).
type Handler func(ctx context.Context, event map[string]any, runCtx *Context) (map[string]any, error)

// Func adapts a simple func(event, context) response so it can be used as a Handler.
func Func(fn func(map[string]any, *Context) map[string]any) Handler {
	return func(_ context.Context, event map[string]any, runCtx *Context) (map[string]any, error) {
		return fn(event, runCtx), nil
	}
}
