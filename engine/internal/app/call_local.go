package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
)

// CallLocal runs the service locally: starts an HTTP server that can invoke handlers.
// Use --serve to keep the server running; without it, a one-off request can be made.
func CallLocal(configPath, stage, host, port string, serve bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	// Phase 14.3: when aiWorkflow is enabled, compile and expose in response (replay not yet implemented).
	var aiWorkflowInfo map[string]string
	if ctx.Config.AiWorkflow != nil && ctx.Config.AiWorkflow.Enable {
		if g, err := config.CompileAiWorkflow(ctx.Config.AiWorkflow); err == nil && g != nil {
			aiWorkflowInfo = map[string]string{"entrypoint": g.Entrypoint, "hash": g.Hash}
		}
	}

	addr := host + ":" + port
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		msg := fmt.Sprintf(`{"message":"RunFabric call-local","service":%q,"stage":%q,"functions":%d`, ctx.Config.Service, ctx.Stage, len(ctx.Config.Functions))
		if len(aiWorkflowInfo) > 0 {
			msg += fmt.Sprintf(`,"aiWorkflow":{"entrypoint":%q,"hash":%q}`, aiWorkflowInfo["entrypoint"], aiWorkflowInfo["hash"])
		}
		_, _ = fmt.Fprint(w, msg+"}")
	})
	for name := range ctx.Config.Functions {
		fnName := name
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"message":"invoke local","function":%q}`, fnName)
		})
	}

	if !serve {
		out := map[string]string{"message": "Use --serve to start local server and run code locally"}
		if len(aiWorkflowInfo) > 0 {
			out["aiWorkflowEntrypoint"] = aiWorkflowInfo["entrypoint"]
			out["aiWorkflowHash"] = aiWorkflowInfo["hash"]
		}
		return out, nil
	}

	fmt.Printf("→ Dev server listening on http://%s (stage=%q)\n", addr, stage)
	if len(aiWorkflowInfo) > 0 {
		fmt.Printf("  aiWorkflow: entrypoint=%s hash=%s\n", aiWorkflowInfo["entrypoint"], aiWorkflowInfo["hash"])
	}
	server := &http.Server{Addr: addr, Handler: mux}
	_ = server.ListenAndServe() // blocks until server stops
	return map[string]string{"addr": addr, "stage": stage}, nil
}

// CallLocalServe starts the dev server in a goroutine and returns a channel that is closed when the server is shut down.
// The returned "restart" function shuts down the server and closes the channel so the caller can restart.
func CallLocalServe(configPath, stage, host, port string) (shutdownChan <-chan struct{}, restart func(), err error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, nil, err
	}

	var aiWorkflowInfo map[string]string
	if ctx.Config.AiWorkflow != nil && ctx.Config.AiWorkflow.Enable {
		if g, compileErr := config.CompileAiWorkflow(ctx.Config.AiWorkflow); compileErr == nil && g != nil {
			aiWorkflowInfo = map[string]string{"entrypoint": g.Entrypoint, "hash": g.Hash}
		}
	}

	addr := host + ":" + port
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		msg := fmt.Sprintf(`{"message":"RunFabric call-local","service":%q,"stage":%q,"functions":%d`, ctx.Config.Service, ctx.Stage, len(ctx.Config.Functions))
		if len(aiWorkflowInfo) > 0 {
			msg += fmt.Sprintf(`,"aiWorkflow":{"entrypoint":%q,"hash":%q}`, aiWorkflowInfo["entrypoint"], aiWorkflowInfo["hash"])
		}
		_, _ = fmt.Fprint(w, msg+"}")
	})
	for name := range ctx.Config.Functions {
		fnName := name
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"message":"invoke local","function":%q}`, fnName)
		})
	}

	server := &http.Server{Addr: addr, Handler: mux}
	done := make(chan struct{})
	restart = func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}

	go func() {
		fmt.Printf("→ Dev server listening on http://%s (stage=%q)\n", addr, stage)
		if len(aiWorkflowInfo) > 0 {
			fmt.Printf("  aiWorkflow: entrypoint=%s hash=%s\n", aiWorkflowInfo["entrypoint"], aiWorkflowInfo["hash"])
		}
		_ = server.ListenAndServe()
		close(done)
	}()

	return done, restart, nil
}
