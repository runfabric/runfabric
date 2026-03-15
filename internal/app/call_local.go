package app

import (
	"fmt"
	"net/http"
)

// CallLocal runs the service locally: starts an HTTP server that can invoke handlers.
// Use --serve to keep the server running; without it, a one-off request can be made.
func CallLocal(configPath, stage, host, port string, serve bool) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}

	addr := host + ":" + port
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Planned: route by runfabric.yml path/method and execute handler (runtime-specific).
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

	server := &http.Server{Addr: addr, Handler: mux}
	_ = server.ListenAndServe() // blocks until server stops
	return map[string]string{"addr": addr, "stage": stage}, nil
}
