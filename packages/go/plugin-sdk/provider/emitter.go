package provider

import (
	"context"

	"github.com/runfabric/runfabric/plugin-sdk/go/server"
)

// Progress emits a progress message from within a provider method.
// No-op when the context carries no emitter (e.g. in-process built-in call).
func Progress(ctx context.Context, msg string) {
	if e := server.EmitterFromContext(ctx); e != nil {
		e.Progress(msg)
	}
}

// Log emits a structured log line. level is one of: debug, info, warn, error.
func Log(ctx context.Context, level, line string) {
	if e := server.EmitterFromContext(ctx); e != nil {
		e.Log(level, line)
	}
}

// Warn emits a warning message.
func Warn(ctx context.Context, msg string) {
	if e := server.EmitterFromContext(ctx); e != nil {
		e.Warn(msg)
	}
}
