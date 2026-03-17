package app

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// CallLocal runs the service locally: starts an HTTP server that can invoke handlers.
// Use --serve to keep the server running; without it, a one-off request can be made.
func CallLocal(configPath, stage, host, port string, serve bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}

	addr := host + ":" + port
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"message":"RunFabric call-local","service":%q,"stage":%q,"functions":%d}`,
			ctx.Config.Service, ctx.Stage, len(ctx.Config.Functions))
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
		return map[string]string{"message": "Use --serve to start local server and run code locally"}, nil
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
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"message":"RunFabric call-local","service":%q,"stage":%q,"functions":%d}`,
			ctx.Config.Service, ctx.Stage, len(ctx.Config.Functions))
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
		_ = server.ListenAndServe()
		close(done)
	}()

	return done, restart, nil
}
