package admin

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

// dashboardLogEntry is one line in the dashboard activity log (terminal + UI).
type dashboardLogEntry struct {
	Time    string `json:"time"`
	Method  string `json:"method"`
	Path    string `json:"path,omitempty"`
	Action  string `json:"action,omitempty"`
	Stage   string `json:"stage,omitempty"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type dashboardLogger struct {
	out     io.Writer
	entries []dashboardLogEntry
	mu      sync.Mutex
	max     int
}

func (d *dashboardLogger) logRequest(method, path string, status int) {
	if d.out == nil {
		return
	}
	log.New(d.out, "[dashboard] ", 0).Printf("%s %s %d", method, path, status)
}

func (d *dashboardLogger) logAction(action, stage string, ok bool, message string) {
	entry := dashboardLogEntry{
		Time:    time.Now().Format("15:04:05"),
		Action:  action,
		Stage:   stage,
		OK:      ok,
		Message: message,
	}
	d.mu.Lock()
	d.entries = append(d.entries, entry)
	if d.max > 0 && len(d.entries) > d.max {
		d.entries = d.entries[len(d.entries)-d.max:]
	}
	d.mu.Unlock()
	if d.out != nil {
		status := "ok"
		if !ok {
			status = "err"
		}
		log.New(d.out, "[dashboard] ", 0).Printf("%s stage=%s %s %s", action, stage, status, message)
	}
}

func (d *dashboardLogger) appendRequest(method, path string) {
	entry := dashboardLogEntry{Time: time.Now().Format("15:04:05"), Method: method, Path: path}
	d.mu.Lock()
	d.entries = append(d.entries, entry)
	if d.max > 0 && len(d.entries) > d.max {
		d.entries = d.entries[len(d.entries)-d.max:]
	}
	d.mu.Unlock()
}

func (d *dashboardLogger) list() []dashboardLogEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]dashboardLogEntry, len(d.entries))
	copy(out, d.entries)
	// newest last for UI
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// recoverHandler wraps h and returns 500 JSON on panic so the dashboard never returns non-JSON or crashes.
func recoverHandler(h http.Handler, out io.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if out != nil {
					log.New(out, "[dashboard] ", 0).Printf("panic: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				msg := "Internal Server Error"
				if s, ok := err.(string); ok && s != "" {
					msg = s
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
			}
		}()
		h.ServeHTTP(w, r)
	})
}

func newDashboardCmd(opts *common.GlobalOptions) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open a local dashboard for project and deploy status",
		Long:  "Starts a local web server that shows the current project, stage selector, and last deploy status (from receipt). Default port 3000.",
		RunE: func(c *cobra.Command, args []string) error {
			stage := opts.Stage
			if stage == "" {
				stage = "dev"
			}
			service := opts.AppService
			if service == nil {
				service = common.NewAppService()
			}
			data, err := app.Dashboard(opts.ConfigPath, stage)
			if err != nil {
				return err
			}
			configPath := opts.ConfigPath
			out := c.OutOrStdout()
			dlog := &dashboardLogger{out: out, max: 100}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/logs", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(dlog.list())
			})
			mux.HandleFunc("POST /action/doctor", func(w http.ResponseWriter, r *http.Request) {
				stage := r.URL.Query().Get("stage")
				if stage == "" {
					stage = "dev"
				}
				result, err := app.BackendDoctor(configPath, stage)
				writeDashboardActionJSON(w, result, err)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				dlog.logAction("doctor", stage, err == nil, msg)
			})
			mux.HandleFunc("POST /action/plan", func(w http.ResponseWriter, r *http.Request) {
				stage := r.URL.Query().Get("stage")
				if stage == "" {
					stage = "dev"
				}
				result, err := service.Plan(configPath, stage, "")
				writeDashboardActionJSON(w, result, err)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				dlog.logAction("plan", stage, err == nil, msg)
			})
			mux.HandleFunc("POST /action/deploy", func(w http.ResponseWriter, r *http.Request) {
				stage := r.URL.Query().Get("stage")
				if stage == "" {
					stage = "dev"
				}
				result, err := service.Deploy(configPath, stage, "", false, false, nil, "")
				writeDashboardActionJSON(w, result, err)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				dlog.logAction("deploy", stage, err == nil, msg)
			})
			mux.HandleFunc("POST /action/remove", func(w http.ResponseWriter, r *http.Request) {
				stage := r.URL.Query().Get("stage")
				if stage == "" {
					stage = "dev"
				}
				result, err := service.Remove(configPath, stage, "")
				writeDashboardActionJSON(w, result, err)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				dlog.logAction("remove", stage, err == nil, msg)
			})
			mux.HandleFunc("POST /action/unlock", func(w http.ResponseWriter, r *http.Request) {
				stage := r.URL.Query().Get("stage")
				if stage == "" {
					stage = "dev"
				}
				result, err := app.Unlock(configPath, stage, true)
				writeDashboardActionJSON(w, result, err)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				dlog.logAction("unlock", stage, err == nil, msg)
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				stageParam := r.URL.Query().Get("stage")
				d := data
				if stageParam != "" {
					d, _ = app.Dashboard(opts.ConfigPath, stageParam)
					if d != nil {
						d.Stage = stageParam
					}
				}
				if d == nil {
					http.Error(w, "failed to load dashboard data", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				appOrgBlock := ""
				if d.App != "" || d.Org != "" {
					appOrgBlock = fmt.Sprintf("<p class=\"meta\">App: %s · Org: %s</p>",
						html.EscapeString(d.App), html.EscapeString(d.Org))
				}
				stagesBlock := ""
				if len(d.Stages) > 0 {
					stagesBlock = "<div class=\"card\"><p class=\"card-title\">Stages</p><div class=\"stages\">"
					for _, e := range d.Stages {
						stagesBlock += fmt.Sprintf("<a href=\"/?stage=%s\">%s</a>", html.EscapeString(e.Stage), html.EscapeString(e.Stage))
					}
					stagesBlock += "</div></div>"
				}
				deployBlock := "<p class=\"none\">No deployment for this stage yet.</p>"
				if d.HasDeployment && d.Receipt != nil {
					deployBlock = fmt.Sprintf(
						"<p class=\"meta\">Deployment: <code>%s</code> · Updated: %s</p>",
						html.EscapeString(d.Receipt.DeploymentID),
						html.EscapeString(d.Receipt.UpdatedAt),
					)
					if len(d.Receipt.Outputs) > 0 {
						deployBlock += "<dl class=\"outputs\">"
						for k, v := range d.Receipt.Outputs {
							deployBlock += fmt.Sprintf("<dt>%s</dt><dd>%s</dd>", html.EscapeString(k), html.EscapeString(v))
						}
						deployBlock += "</dl>"
					}
				}
				workflowBlock := "<div class=\"card\"><p class=\"card-title\">Workflows</p>"
				if d.WorkflowRunCount > 0 {
					workflowBlock += fmt.Sprintf("<p class=\"meta\">Runs: %d</p>", d.WorkflowRunCount)
					if d.WorkflowCost != nil {
						workflowBlock += fmt.Sprintf("<p class=\"meta\">Input tokens: %d · Output tokens: %d · Est. cost: $%.4f</p>",
							d.WorkflowCost.TotalInputTokens, d.WorkflowCost.TotalOutputTokens, d.WorkflowCost.EstimatedCostUSD)
					}
				} else {
					workflowBlock += "<p class=\"none\">No workflow runs yet. Use <code>runfabric workflow run</code>.</p>"
				}
				if d.RouterHistory != nil && d.RouterHistory.Total.Snapshots > 0 {
					workflowBlock += "<hr style=\"border:0;border-top:1px solid #eef2f7;margin:0.75rem 0;\"/>"
					workflowBlock += "<p class=\"card-title\">Router Observability</p>"
					workflowBlock += fmt.Sprintf(
						"<p class=\"meta\">Snapshots: %d · Drift: %d · Mutation trend: %s</p>",
						d.RouterHistory.Total.Snapshots,
						d.RouterHistory.Total.Drift,
						html.EscapeString(d.RouterHistory.Trend),
					)
					if d.RouterHistory.LastOperation != "" || d.RouterHistory.LastSnapshotAt != "" {
						workflowBlock += fmt.Sprintf(
							"<p class=\"meta\">Last op: <code>%s</code> · At: %s</p>",
							html.EscapeString(d.RouterHistory.LastOperation),
							html.EscapeString(d.RouterHistory.LastSnapshotAt),
						)
					}
				}
				workflowBlock += "</div>"
				_, _ = fmt.Fprintf(w, common.DashboardHTML, d.Service, d.Service, d.Stage, appOrgBlock, stagesBlock, deployBlock, workflowBlock)
			})
			// Attach logging middleware to record terminal output and GET / activity entries.
			loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rw := &responseWriter{ResponseWriter: w, status: 200}
				mux.ServeHTTP(rw, r)
				dlog.logRequest(r.Method, r.URL.Path, rw.status)
				if r.Method == "GET" && (r.URL.Path == "/" || r.URL.Path == "") {
					dlog.appendRequest(r.Method, r.URL.Path)
				}
			})
			addr := ":" + strconv.Itoa(port)
			url := "http://localhost" + addr
			fmt.Fprintf(out, "\n  Dashboard: %s\n\n  Press Ctrl+C to stop the server.\n\n", url)
			return http.ListenAndServe(addr, recoverHandler(loggingHandler, out))
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 3000, "Port for the dashboard server")
	return cmd
}

func writeDashboardActionJSON(w http.ResponseWriter, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}
