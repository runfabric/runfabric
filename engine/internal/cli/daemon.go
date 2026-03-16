package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/runfabric/runfabric/engine/internal/configapi"
	"github.com/runfabric/runfabric/engine/internal/telemetry"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// daemonOtelMiddleware creates a span per request when OpenTelemetry is configured.
func daemonOtelMiddleware(tr trace.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tr.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
		)
		r = r.WithContext(ctx)
		rec := &otelResponseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status >= 400 {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
			span.SetAttributes(attribute.Int("http.status_code", rec.status))
		}
	})
}

type otelResponseRecorder struct {
	http.ResponseWriter
	status int
}

func (o *otelResponseRecorder) WriteHeader(code int) {
	o.status = code
	o.ResponseWriter.WriteHeader(code)
}

func newDaemonCmd(opts *GlobalOptions) *cobra.Command {
	var address string
	var port int
	var apiKey string
	var rateLimit int
	var withDashboard bool
	var workspace string
	var cacheTTL time.Duration
	var cacheURL string

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run a long-running API server (config API + optional dashboard)",
		Long:  "Starts a single process serving the config API (POST /validate, /resolve, /plan, /deploy, /remove, /releases) and optionally the dashboard at GET /. Use --dashboard and ensure --config points to a runfabric.yml workspace. Optional --api-key, --rate-limit, --workspace. Suitable for foreground use or as an OS service (systemd, launchd): run the binary with --config and optionally --workspace so state paths are resolved from that directory.",
		RunE: func(c *cobra.Command, args []string) error {
			stage := opts.Stage
			if stage == "" {
				stage = "dev"
			}
			configPath := opts.ConfigPath
			if workspace != "" {
				configPath = filepath.Join(workspace, configPath)
			}
			if withDashboard && configPath == "" {
				return fmt.Errorf("--dashboard requires --config (path to runfabric.yml)")
			}

			srv := configapi.NewServer(stage)
			srv.APIKey = apiKey
			srv.RateLimitN = rateLimit
			apiHandler := srv.Handler()

			cacheURLVal := cacheURL
			if cacheURLVal == "" {
				cacheURLVal = os.Getenv("RUNFABRIC_DAEMON_CACHE_URL")
			}
			cacheURLVal = strings.TrimSpace(cacheURLVal)
			usingRedis := cacheURLVal != "" && (strings.HasPrefix(cacheURLVal, "redis://") || strings.HasPrefix(cacheURLVal, "rediss://"))

			// Distributed API cache (validate, resolve, plan, releases) when --cache-url is Redis
			var apiCache *daemonAPICache
			if usingRedis {
				apiTTL := cacheTTL
				if apiTTL <= 0 {
					apiTTL = 5 * time.Minute
				}
				apiCache = newDaemonAPICache(cacheURLVal, apiTTL)
				if apiCache != nil {
					apiHandler = apiCacheMiddleware(apiCache, stage, apiHandler)
				}
			}

			mux := http.NewServeMux()
			// Config API: POST /validate, /resolve, /plan, /deploy, /remove, /releases (wrapped with API cache when cache-url set)
			mux.HandleFunc("POST /validate", apiHandler.ServeHTTP)
			mux.HandleFunc("POST /resolve", apiHandler.ServeHTTP)
			mux.HandleFunc("POST /plan", apiHandler.ServeHTTP)
			mux.HandleFunc("POST /deploy", apiHandler.ServeHTTP)
			mux.HandleFunc("POST /remove", apiHandler.ServeHTTP)
			mux.HandleFunc("POST /releases", apiHandler.ServeHTTP)

			if withDashboard {
				mux.HandleFunc("POST /action/plan", func(w http.ResponseWriter, r *http.Request) {
					st := r.URL.Query().Get("stage")
					if st == "" {
						st = "dev"
					}
					result, err := app.Plan(configPath, st, "")
					writeDaemonActionJSON(w, result, err)
				})
				mux.HandleFunc("POST /action/deploy", func(w http.ResponseWriter, r *http.Request) {
					st := r.URL.Query().Get("stage")
					if st == "" {
						st = "dev"
					}
					result, err := app.Deploy(configPath, st, "", false, false, nil, "")
					if err == nil && apiCache != nil {
						apiCache.invalidateStage(st)
					}
					writeDaemonActionJSON(w, result, err)
				})
				mux.HandleFunc("POST /action/remove", func(w http.ResponseWriter, r *http.Request) {
					st := r.URL.Query().Get("stage")
					if st == "" {
						st = "dev"
					}
					result, err := app.Remove(configPath, st, "")
					if err == nil && apiCache != nil {
						apiCache.invalidateStage(st)
					}
					writeDaemonActionJSON(w, result, err)
				})
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/" {
						http.NotFound(w, r)
						return
					}
					stageParam := r.URL.Query().Get("stage")
					st := stage
					if stageParam != "" {
						st = stageParam
					}
					d, err := app.Dashboard(configPath, st)
					if err != nil || d == nil {
						http.Error(w, "failed to load dashboard data", http.StatusInternalServerError)
						return
					}
					d.Stage = st
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					stagesBlock := ""
					if len(d.Stages) > 0 {
						stagesBlock = "<div class=\"stages\">Stages: "
						for _, e := range d.Stages {
							stagesBlock += fmt.Sprintf("<a href=\"/?stage=%s\">%s</a> ", e.Stage, e.Stage)
						}
						stagesBlock += "</div>"
					}
					deployBlock := "<p class=\"none\">No deployment for this stage yet.</p>"
					if d.HasDeployment && d.Receipt != nil {
						deployBlock = fmt.Sprintf(
							"<p class=\"meta\">Deployment: <code>%s</code> · Updated: %s</p>",
							d.Receipt.DeploymentID,
							d.Receipt.UpdatedAt,
						)
						if len(d.Receipt.Outputs) > 0 {
							deployBlock += "<dl class=\"outputs\">"
							for k, v := range d.Receipt.Outputs {
								deployBlock += fmt.Sprintf("<dt>%s</dt><dd>%s</dd>", k, v)
							}
							deployBlock += "</dl>"
						}
					}
					appOrgBlock := ""
					if d.App != "" || d.Org != "" {
						appOrgBlock = fmt.Sprintf("<p class=\"meta\">App: %s · Org: %s</p>",
							html.EscapeString(d.App), html.EscapeString(d.Org))
					}
					_, _ = fmt.Fprintf(w, dashboardHTML, d.Service, d.Service, d.Stage, appOrgBlock, stagesBlock, deployBlock)
				})
			} else {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/" {
						http.NotFound(w, r)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"service":   "runfabric-daemon",
						"api":       "POST /validate, /resolve, /plan, /deploy, /remove, /releases",
						"dashboard": "run with --dashboard and --config for GET /",
					})
				})
			}

			addr := address + ":" + strconv.Itoa(port)
			fmt.Fprintf(c.OutOrStdout(), "Daemon listening on http://%s\n", addr)
			if withDashboard {
				fmt.Fprintf(c.OutOrStdout(), "  Dashboard: GET /\n")
			}
			if apiCache != nil {
				fmt.Fprintf(c.OutOrStdout(), "  API cache: distributed (Redis), validate/resolve/plan/releases\n")
			}
			fmt.Fprintf(c.OutOrStdout(), "  API: POST /validate, /resolve, /plan, /deploy, /remove, /releases\n")
			handler := daemonOtelMiddleware(telemetry.Tracer("runfabric/daemon"), mux)
			return http.ListenAndServe(addr, handler)
		},
	}

	cmd.Flags().StringVar(&address, "address", "0.0.0.0", "Listen address")
	cmd.Flags().IntVarP(&port, "port", "p", 8766, "Listen port (default 8766 to avoid conflict with config-api)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Optional: require X-API-Key header")
	cmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "Optional: max requests per minute per client (0 = disabled)")
	cmd.Flags().BoolVar(&withDashboard, "dashboard", false, "Serve dashboard at GET / (requires --config)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Project root directory; --config is resolved relative to this (e.g. for systemd/launchd: WorkingDirectory=... and --workspace .)")
	cmd.Flags().DurationVar(&cacheTTL, "cache-ttl", 5*time.Minute, "API cache TTL when --cache-url is set (e.g. 5m); 0 uses per-endpoint defaults")
	cmd.Flags().StringVar(&cacheURL, "cache-url", "", "Distributed cache URL (e.g. redis://localhost:6379/0). Caches Config API (validate, resolve, plan, releases). Env: RUNFABRIC_DAEMON_CACHE_URL.")
	return cmd
}

func writeDaemonActionJSON(w http.ResponseWriter, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}
