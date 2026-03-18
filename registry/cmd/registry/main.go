package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/runfabric/runfabric/registry/internal/resolve"
)

func main() {
	var listen string
	flag.StringVar(&listen, "listen", "127.0.0.1:8787", "host:port to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/v1/extensions/resolve", resolve.NewHandler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":      "NOT_FOUND",
				"message":   "Not found",
				"requestId": r.Header.Get("X-Request-Id"),
			},
		})
	})

	s := &http.Server{
		Addr:              listen,
		Handler:           logRequests(withRequestID(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("registry listening on http://%s", listen)
	log.Fatal(s.ListenAndServe())
}

var reqSeq uint64

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("X-Request-Id")) == "" {
			seq := atomic.AddUint64(&reqSeq, 1)
			r.Header.Set("X-Request-Id", "req_local_"+runtime.GOOS+"_"+time.Now().UTC().Format("20060102T150405.000Z0700")+"_"+strconv.FormatUint(seq, 10))
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
