package daemoncmd

import (
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"
	daemonserver "github.com/runfabric/runfabric/platform/daemon/server"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

// NewDaemonCmd returns the daemon command with the provided Use token.
// Typical value is "runfabricd" for the daemon binary root.
func NewDaemonCmd(opts *common.GlobalOptions, use string) *cobra.Command {
	use = strings.TrimSpace(use)
	if use == "" {
		use = "runfabricd"
	}

	var address string
	var port int
	var apiKey string
	var rateLimit int
	var withDashboard bool
	var workspace string
	var cacheTTL time.Duration
	var cacheURL string

	cmd := &cobra.Command{
		Use:   use,
		Short: "Run a long-running API server (config API + optional dashboard)",
		Long:  "Starts a single process serving the config API (POST /validate, /resolve, /plan, /deploy, /remove, /releases) and optionally the dashboard at GET /. Use --dashboard and ensure --config points to a runfabric.yml workspace. Optional --api-key, --rate-limit, --workspace. Suitable for foreground use or as an OS service (systemd, launchd): run the binary with --config and optionally --workspace so state paths are resolved from that directory.",
		RunE: func(c *cobra.Command, args []string) error {
			stage := opts.Stage
			if stage == "" {
				stage = "dev"
			}
			service := opts.AppService
			if service == nil {
				service = common.NewAppService()
			}
			configPath := opts.ConfigPath
			if workspace != "" {
				configPath = filepath.Join(workspace, configPath)
			}
			if withDashboard && configPath == "" {
				return fmt.Errorf("--dashboard requires --config (path to runfabric.yml)")
			}

			srv := daemonserver.New(daemonserver.Options{
				Address:   address,
				Port:      port,
				Stage:     stage,
				APIKey:    apiKey,
				RateLimit: rateLimit,
				CacheURL:  cacheURL,
				CacheTTL:  cacheTTL,
			})

			handler := srv.Handler(func(mux *http.ServeMux) {
				if withDashboard {
					mux.HandleFunc("POST /action/plan", func(w http.ResponseWriter, r *http.Request) {
						st := r.URL.Query().Get("stage")
						if st == "" {
							st = "dev"
						}
						result, err := service.Plan(configPath, st, "")
						writeDaemonActionJSON(w, result, err)
					})
					mux.HandleFunc("POST /action/deploy", func(w http.ResponseWriter, r *http.Request) {
						st := r.URL.Query().Get("stage")
						if st == "" {
							st = "dev"
						}
						result, err := service.Deploy(configPath, st, "", false, false, nil, "")
						if err == nil {
							srv.InvalidateStage(st)
						}
						writeDaemonActionJSON(w, result, err)
					})
					mux.HandleFunc("POST /action/remove", func(w http.ResponseWriter, r *http.Request) {
						st := r.URL.Query().Get("stage")
						if st == "" {
							st = "dev"
						}
						result, err := service.Remove(configPath, st, "")
						if err == nil {
							srv.InvalidateStage(st)
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
						workflowBlock += "</div>"
						_, _ = fmt.Fprintf(w, common.DashboardHTML, d.Service, d.Service, d.Stage, appOrgBlock, stagesBlock, deployBlock, workflowBlock)
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
			})

			addr := srv.Addr()
			if sockPath := daemonserver.ListenOnSocket(handler); sockPath != "" {
				fmt.Fprintf(c.OutOrStdout(), "  Unix socket: %s\n", sockPath)
			}
			fmt.Fprintf(c.OutOrStdout(), "Daemon listening on http://%s\n", addr)
			if withDashboard {
				fmt.Fprintf(c.OutOrStdout(), "  Dashboard: GET /\n")
			}
			if srv.UsingCache() {
				fmt.Fprintf(c.OutOrStdout(), "  API cache: distributed (Redis), validate/resolve/plan/releases\n")
			}
			fmt.Fprintf(c.OutOrStdout(), "  API: POST /validate, /resolve, /plan, /deploy, /remove, /releases\n")
			fmt.Fprintf(c.OutOrStdout(), "  Health: GET /healthz  Version: GET /version\n")
			if err := http.ListenAndServe(addr, handler); err != nil {
				if strings.Contains(err.Error(), "address already in use") {
					fmt.Fprintf(os.Stderr, "Error: %v - try: runfabricd stop (or use --port to pick another port)\n", err)
					os.Exit(1)
				}
				return err
			}
			return nil
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

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start the daemon in the background",
			Long:  "Spawns the daemon as a detached process. PID is written to .runfabric/daemon.pid and logs to .runfabric/daemon.log. Run from project root so start/stop use the same .runfabric directory.",
			RunE:  runDaemonStart,
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the daemon started with runfabricd start",
			Long:  "Sends SIGTERM to the process whose PID is in .runfabric/daemon.pid and removes the file. Run from the same directory used for runfabricd start.",
			RunE:  runDaemonStop,
		},
		&cobra.Command{
			Use:   "restart",
			Short: "Stop the daemon (if running) and start it again in the background",
			Long:  "Equivalent to runfabricd stop followed by runfabricd start. Uses the same .runfabric directory; run from project root.",
			RunE:  runDaemonRestart,
		},
		&cobra.Command{
			Use:   "status",
			Short: "Report whether the daemon is running (from .runfabric/daemon.pid)",
			Long:  "Reads .runfabric/daemon.pid and checks if that process is alive. Run from the same directory used for runfabricd start.",
			RunE:  runDaemonStatus,
		},
	)
	return cmd
}

// daemonPortResponding returns true if something is listening on host:port (e.g. the daemon).
func daemonPortResponding(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func daemonDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".runfabric"), nil
}

func runDaemonStatus(c *cobra.Command, _ []string) error {
	dir, err := daemonDir()
	if err != nil {
		return err
	}
	pidPath := filepath.Join(dir, "daemon.pid")
	b, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			if daemonPortResponding("127.0.0.1", 8766) {
				fmt.Fprintf(c.OutOrStdout(), "daemon: no pid file but something is listening on http://127.0.0.1:8766 (daemon may still be running; use 'lsof -i :8766' to find PID, then kill)\n")
			} else {
				fmt.Fprintf(c.OutOrStdout(), "daemon: not running (no .runfabric/daemon.pid)\n")
			}
			return nil
		}
		return fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		fmt.Fprintf(c.OutOrStdout(), "daemon: not running (invalid pid file)\n")
		_ = os.Remove(pidPath)
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(c.OutOrStdout(), "daemon: not running (stale pid file)\n")
		_ = os.Remove(pidPath)
		return nil
	}
	if runtime.GOOS != "windows" {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			fmt.Fprintf(c.OutOrStdout(), "daemon: not running (PID %d gone, removed stale pid file)\n", pid)
			_ = os.Remove(pidPath)
			return nil
		}
	}
	fmt.Fprintf(c.OutOrStdout(), "daemon: running (PID %d)\n", pid)
	return nil
}

func runDaemonRestart(c *cobra.Command, args []string) error {
	_ = runDaemonStop(c, nil)
	time.Sleep(1 * time.Second)
	return runDaemonStart(c, nil)
}

func runDaemonStart(c *cobra.Command, _ []string) error {
	dir, err := daemonDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create .runfabric: %w", err)
	}
	logPath := filepath.Join(dir, "daemon.log")
	pidPath := filepath.Join(dir, "daemon.pid")

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable path: %w", err)
	}
	argv := os.Args[1:]
	var newArgs []string
	for _, a := range argv {
		if a == "start" || a == "restart" {
			continue
		}
		newArgs = append(newArgs, a)
	}

	cmd := exec.Command(execPath, newArgs...)
	cmd.Stdin = nil
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = filepath.Dir(dir)
	configureDaemonChildProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("write pid file: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "Daemon started (PID %d). Logs: %s\n", pid, logPath)
	return nil
}

func runDaemonStop(c *cobra.Command, _ []string) error {
	dir, err := daemonDir()
	if err != nil {
		return err
	}
	pidPath := filepath.Join(dir, "daemon.pid")
	b, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(c.OutOrStderr(), "No daemon PID file at %s (daemon may not be running).\n", pidPath)
			return nil
		}
		return fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return fmt.Errorf("invalid pid file: %w", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	if runtime.GOOS == "windows" {
		if err := proc.Kill(); err != nil {
			if strings.Contains(err.Error(), "already finished") || err == syscall.ESRCH {
				_ = os.Remove(pidPath)
				fmt.Fprintf(c.OutOrStdout(), "Daemon (PID %d) not running. Removed stale pid file.\n", pid)
				return nil
			}
			return fmt.Errorf("kill daemon: %w", err)
		}
	} else {
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			if strings.Contains(err.Error(), "process already finished") || err == syscall.ESRCH {
				_ = os.Remove(pidPath)
				fmt.Fprintf(c.OutOrStdout(), "Daemon (PID %d) not running. Removed stale pid file.\n", pid)
				return nil
			}
			return fmt.Errorf("signal daemon: %w", err)
		}
	}
	_ = os.Remove(pidPath)
	fmt.Fprintf(c.OutOrStdout(), "Daemon stopped (PID %d).\n", pid)
	return nil
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
