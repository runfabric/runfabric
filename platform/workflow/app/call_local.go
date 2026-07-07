package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// CallLocal runs the service locally: starts an HTTP server that can invoke handlers.
// Use --serve to keep the server running; without it, a one-off request can be made.
func CallLocal(configPath, stage, host, port string, serve bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	addr := host + ":" + port
	simID, err := EnsureLocalSimulator(ctx)
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
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
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
	simID, err := EnsureLocalSimulator(ctx)
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

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
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

// EnsureLocalSimulator resolves the configured (or default "local") simulator
// plugin and ensures it is loaded on the context, returning its id. Shared by
// the dev server (call-local) and one-shot invocation (invoke-local).
func EnsureLocalSimulator(ctx *AppContext) (string, error) {
	simID := resolveSimulatorIDForLocal(ctx)
	if simID == "" {
		simID = "local"
	}
	if err := ctx.Extensions.EnsureSimulator(simID); err != nil {
		return "", err
	}
	return simID, nil
}

// LocalInvokeRequest is one HTTP-shaped invocation of a function against a local
// simulator (no cloud deploy). Empty Method/Path default to GET "/".
type LocalInvokeRequest struct {
	Method  string
	Path    string
	Query   map[string]string
	Headers map[string]string
	Body    []byte
}

// LocalInvokeResponse is the simulator's response for a single local invocation.
// Body is the raw response payload (often JSON) as returned by the handler.
type LocalInvokeResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// SimulateOne invokes a single function once through the given simulator and
// returns its response. It is the shared primitive behind both the dev server's
// per-request handler and the daemon's one-shot POST /invoke-local: it reads the
// function's handler ref + runtime from the resolved config and hands the
// simulator the on-disk WorkDir so it can actually execute the handler.
func SimulateOne(ctx *AppContext, simulatorID, fnName string, in LocalInvokeRequest) (*LocalInvokeResponse, error) {
	// A zero-value fnCfg (function absent from the config) yields the simulator's
	// echo fallback rather than an error; callers that require the function to
	// exist (e.g. the daemon route) validate its presence before calling.
	fnCfg := ctx.Config.Functions[fnName]
	runtime := fnCfg.Runtime
	if runtime == "" {
		runtime = ctx.Config.Provider.Runtime
	}
	method := in.Method
	if method == "" {
		method = http.MethodGet
	}
	path := in.Path
	if path == "" {
		path = "/"
	}
	res, err := ctx.Extensions.Simulate(context.Background(), simulatorID, SimulatorInvokeRequest{
		Service:    ctx.Config.Service,
		Stage:      ctx.Stage,
		Function:   fnName,
		Method:     method,
		Path:       path,
		Query:      in.Query,
		Headers:    in.Headers,
		Body:       in.Body,
		WorkDir:    ctx.RootDir,
		HandlerRef: fnCfg.Handler,
		Runtime:    runtime,
	})
	if err != nil {
		return nil, err
	}
	status := res.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	return &LocalInvokeResponse{StatusCode: status, Headers: res.Headers, Body: res.Body}, nil
}

func newLocalInvokeHandler(ctx *AppContext, simulatorID, fnName string) http.HandlerFunc {
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
		res, err := SimulateOne(ctx, simulatorID, fnName, LocalInvokeRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   query,
			Headers: headers,
			Body:    body,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"error":%q,"function":%q}`, err.Error(), fnName)
			return
		}
		for k, v := range res.Headers {
			w.Header().Set(k, v)
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(res.StatusCode)
		if len(res.Body) == 0 {
			payload, _ := json.Marshal(map[string]any{"message": "invoke local", "function": fnName})
			_, _ = w.Write(payload)
			return
		}
		_, _ = w.Write(res.Body)
	}
}
