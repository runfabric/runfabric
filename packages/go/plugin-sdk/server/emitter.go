package server

import (
	"bufio"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/runfabric/runfabric/plugin-sdk/go/protocol"
)

type emitterKey struct{}

// Emitter pushes streaming events to the engine before the final result.
// It is safe to call from any goroutine.
type Emitter struct {
	requestID string
	mu        sync.Mutex
	w         *bufio.Writer
}

func newEmitter(requestID string, w *bufio.Writer) *Emitter {
	return &Emitter{requestID: requestID, w: w}
}

// Progress emits a progress message (e.g. "Building image…").
func (e *Emitter) Progress(msg string) {
	e.emit(protocol.Event{Type: "progress", Message: msg})
}

// Log emits a structured log line. level is one of debug, info, warn, error.
func (e *Emitter) Log(level, line string) {
	e.emit(protocol.Event{
		Type:      "log",
		Level:     level,
		Line:      line,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Warn emits a warning message.
func (e *Emitter) Warn(msg string) {
	e.emit(protocol.Event{Type: "warn", Message: msg})
}

func (e *Emitter) emit(ev protocol.Event) {
	ev.RequestID = e.requestID
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.w.Write(append(b, '\n'))
	_ = e.w.Flush()
}

// contextWithEmitter returns a new context carrying the emitter.
func contextWithEmitter(ctx context.Context, e *Emitter) context.Context {
	return context.WithValue(ctx, emitterKey{}, e)
}

// EmitterFromContext returns the Emitter stored in ctx, or nil if none.
func EmitterFromContext(ctx context.Context) *Emitter {
	e, _ := ctx.Value(emitterKey{}).(*Emitter)
	return e
}
