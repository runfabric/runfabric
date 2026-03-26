package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

// CallLocal runs the service locally: starts an HTTP server that can invoke handlers.
// Use --serve to keep the server running; without it, a one-off request can be made.
func CallLocal(configPath, stage, host, port string, serve bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	addr := host + ":" + port
	simID := resolveSimulatorIDForLocal(ctx)
	if simID == "" {
		simID = "local"
	}
	_, err = ctx.Extensions.ResolveSimulator(simID)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		msg := fmt.Sprintf(`{"message":"RunFabric call-local","service":%q,"stage":%q,"functions":%d}`, ctx.Config.Service, ctx.Stage, len(ctx.Config.Functions))
		_, _ = fmt.Fprint(w, msg)
	})
	for name := range ctx.Config.Functions {
		fnName := name
		mux.HandleFunc("/"+name, newLocalInvokeHandler(ctx, simID, fnName))
	}

	if !serve {
		out := map[string]string{"message": "Use --serve to start local server and run code locally"}
		return out, nil
	}

	fmt.Printf("→ Dev server listening on http://%s (stage=%q)\n", addr, stage)
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

	addr := host + ":" + port
	simID := resolveSimulatorIDForLocal(ctx)
	if simID == "" {
		simID = "local"
	}
	_, err = ctx.Extensions.ResolveSimulator(simID)
	if err != nil {
		return nil, nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		msg := fmt.Sprintf(`{"message":"RunFabric call-local","service":%q,"stage":%q,"functions":%d}`, ctx.Config.Service, ctx.Stage, len(ctx.Config.Functions))
		_, _ = fmt.Fprint(w, msg)
	})
	for name := range ctx.Config.Functions {
		fnName := name
		mux.HandleFunc("/"+name, newLocalInvokeHandler(ctx, simID, fnName))
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
		_ = server.ListenAndServe()
		close(done)
	}()

	return done, restart, nil
}

func resolveSimulatorIDForLocal(ctx *AppContext) string {
	simID := config.ExtensionString(ctx.Config, "simulatorPlugin")
	if simID == "" {
		simID = "local"
	}
	return simID
}

func newLocalInvokeHandler(ctx *AppContext, simulatorID, fnName string) http.HandlerFunc {
	fnCfg := ctx.Config.Functions[fnName]
	runtime := fnCfg.Runtime
	if runtime == "" {
		runtime = ctx.Config.Runtime
	}
	return func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 0)
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
		}
		query := map[string]string{}
		for k := range r.URL.Query() {
			query[k] = r.URL.Query().Get(k)
		}
		headers := map[string]string{}
		for k := range r.Header {
			headers[k] = r.Header.Get(k)
		}
		res, err := ctx.Extensions.Simulate(r.Context(), simulatorID, resolution.SimulatorInvokeRequest{
			Service:    ctx.Config.Service,
			Stage:      ctx.Stage,
			Function:   fnName,
			Method:     r.Method,
			Path:       r.URL.Path,
			Query:      query,
			Headers:    headers,
			Body:       body,
			WorkDir:    ctx.RootDir,
			HandlerRef: fnCfg.Handler,
			Runtime:    runtime,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"error":%q,"function":%q}`, err.Error(), fnName)
			return
		}
		status := res.StatusCode
		if status == 0 {
			status = http.StatusOK
		}
		for k, v := range res.Headers {
			w.Header().Set(k, v)
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if len(res.Body) == 0 {
			payload, _ := json.Marshal(map[string]any{"message": "invoke local", "function": fnName})
			_, _ = w.Write(payload)
			return
		}
		_, _ = w.Write(res.Body)
	}
}
