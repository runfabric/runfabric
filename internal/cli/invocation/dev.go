package invocation

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/platform/observability/diagnostics"
	"github.com/spf13/cobra"
)

func newDevCmd(opts *GlobalOptions) *cobra.Command {
	var host, port, provider, preset, method, path, query, body, header, entry, out, streamFrom, tunnelURL string
	var watch, once, doctorFirst bool
	var intervalSeconds int

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Local dev with optional watch",
		Long:  "Runs your service locally (same as call-local --serve). Use --watch to auto-reload on file changes. Use --stream-from and --tunnel-url for live-stream mode. See docs/DEV_LIVE_STREAM.md.",
		RunE: func(cmd *cobra.Command, args []string) error {
			effectiveStage := opts.Stage
			if streamFrom != "" {
				effectiveStage = streamFrom
			}
			if doctorFirst {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Running doctor preflight for stage=%q...\n", effectiveStage)
				}
				var doctorResult any
				var err error
				if streamFrom != "" && tunnelURL != "" {
					doctorResult, err = app.DevStreamDoctor(opts.ConfigPath, effectiveStage, tunnelURL)
				} else {
					doctorResult, err = app.BackendDoctor(opts.ConfigPath, effectiveStage)
				}
				if err != nil {
					common.StatusFail(opts.JSONOutput, "Doctor preflight failed.")
					return common.PrintFailure("doctor", err)
				}
				if !opts.JSONOutput {
					if report, ok := doctorResult.(*diagnostics.HealthReport); ok {
						for _, check := range report.Checks {
							if strings.HasPrefix(check.Name, "dev-stream-") {
								fmt.Fprintf(cmd.OutOrStdout(), "Doctor %s: %s\n", check.Name, check.Message)
							}
						}
					}
				}
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Doctor preflight passed.\n")
				}
			}
			common.StatusRunning(opts.JSONOutput, "Starting dev server...")

			var restore func()
			var devReport *app.DevStreamReport
			if streamFrom != "" && tunnelURL != "" {
				var err error
				restore, devReport, err = app.PrepareDevStreamTunnelWithReport(opts.ConfigPath, effectiveStage, tunnelURL)
				if err != nil {
					common.StatusFail(opts.JSONOutput, "Dev stream redirect failed.")
					return common.PrintFailure("dev", err)
				}
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Live stream: stage=%q, tunnel=%s — provider hook prepared; state will be restored on exit when applicable.\n", effectiveStage, tunnelURL)
					if devReport != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Dev stream mode: %s (capability: %s). %s\n", devReport.EffectiveMode, devReport.CapabilityMode, devReport.Message)
						if len(devReport.MissingPrereqs) > 0 {
							fmt.Fprintf(cmd.OutOrStdout(), "Missing mutation prerequisites: %s\n", strings.Join(devReport.MissingPrereqs, ", "))
						}
					}
				}
			} else if streamFrom != "" && !opts.JSONOutput {
				fmt.Fprintf(cmd.OutOrStdout(), "Live stream mode: stage=%q, listening on %s:%s\n", effectiveStage, host, port)
				if tunnelURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Tunnel URL: %s — point your provider's invocation target here.\n", tunnelURL)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Start a tunnel (e.g. ngrok http %s) and set your provider to invoke that URL. See docs/DEV_LIVE_STREAM.md.\n", port)
				}
			}

			if restore != nil {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, os.Interrupt)
				go func() {
					_, _ = app.CallLocal(opts.ConfigPath, effectiveStage, host, port, true)
				}()
				<-sigCh
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Restoring live-stream provider state...\n")
				}
				restore()
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Restored. Exiting.\n")
				}
				os.Exit(0)
			}

			// Watch mode: restart server when project files change
			if watch {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, os.Interrupt)
				watchDone := make(chan struct{})
				watchChan := app.WatchProjectDir(opts.ConfigPath, 1*time.Second, watchDone)
				for {
					shutdownChan, restart, err := app.CallLocalServe(opts.ConfigPath, effectiveStage, host, port)
					if err != nil {
						common.StatusFail(opts.JSONOutput, "Dev failed.")
						return common.PrintFailure("dev", err)
					}
					select {
					case <-watchChan:
						if !opts.JSONOutput {
							fmt.Fprintf(cmd.OutOrStdout(), "→ File change detected, reloading...\n")
						}
						restart()
						<-shutdownChan
					case <-sigCh:
						close(watchDone)
						restart()
						<-shutdownChan
						if !opts.JSONOutput {
							fmt.Fprintf(cmd.OutOrStdout(), "Exiting.\n")
						}
						return nil
					}
				}
			}

			result, err := app.CallLocal(opts.ConfigPath, effectiveStage, host, port, true)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Dev failed.")
				return common.PrintFailure("dev", err)
			}
			common.StatusDone(opts.JSONOutput, "Dev server ready.")
			_ = once
			_ = provider
			_ = preset
			_ = method
			_ = path
			_ = query
			_ = body
			_ = header
			_ = entry
			_ = out
			_ = intervalSeconds
			if opts.JSONOutput {
				return common.PrintJSONSuccess("dev", result)
			}
			return common.PrintSuccess("dev", result)
		},
	}

	cmd.Flags().StringVar(&streamFrom, "stream-from", "", "Stage to stream invocations from; runs local server so you can point your provider invocation target at this process via a tunnel")
	cmd.Flags().StringVar(&tunnelURL, "tunnel-url", "", "Public URL of your tunnel (e.g. from ngrok); show in instructions as the invocation target")
	cmd.Flags().BoolVar(&doctorFirst, "doctor-first", false, "Run doctor preflight before starting dev server")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host for local server")
	cmd.Flags().StringVar(&port, "port", "3000", "Port for local server")
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch files and rebuild")
	cmd.Flags().BoolVar(&once, "once", false, "Run once and exit (e.g. single trigger)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider to emulate")
	cmd.Flags().StringVar(&preset, "preset", "", "Preset: http|queue|storage|cron|eventbridge|pubsub|kafka|rabbitmq")
	cmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	cmd.Flags().StringVar(&path, "path", "", "Request path")
	cmd.Flags().StringVar(&query, "query", "", "Query string")
	cmd.Flags().StringVar(&body, "body", "", "Request body")
	cmd.Flags().StringVar(&header, "header", "", "Header (k:v)")
	cmd.Flags().StringVar(&entry, "entry", "", "Entry path override")
	cmd.Flags().StringVar(&out, "out", "", "Output directory")
	cmd.Flags().IntVar(&intervalSeconds, "interval-seconds", 0, "Polling interval in seconds (e.g. for cron)")
	return cmd
}
