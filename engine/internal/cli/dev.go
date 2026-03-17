package cli

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newDevCmd(opts *GlobalOptions) *cobra.Command {
	var host, port, provider, preset, method, path, query, body, header, entry, out, streamFrom, tunnelURL string
	var watch, once bool
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
			statusRunning(opts.JSONOutput, "Starting dev server...")

			var restore func()
			if streamFrom != "" && tunnelURL != "" {
				var err error
				restore, err = app.PrepareDevStreamTunnel(opts.ConfigPath, effectiveStage, tunnelURL)
				if err != nil {
					statusFail(opts.JSONOutput, "Dev stream redirect failed.")
					return printFailure("dev", err)
				}
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Live stream: stage=%q, tunnel=%s — API Gateway pointed at tunnel; will restore on exit.\n", effectiveStage, tunnelURL)
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
					fmt.Fprintf(cmd.OutOrStdout(), "Restoring provider invocation target...\n")
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
						statusFail(opts.JSONOutput, "Dev failed.")
						return printFailure("dev", err)
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
				statusFail(opts.JSONOutput, "Dev failed.")
				return printFailure("dev", err)
			}
			statusDone(opts.JSONOutput, "Dev server ready.")
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
				return printJSONSuccess("dev", result)
			}
			return printSuccess("dev", result)
		},
	}

	cmd.Flags().StringVar(&streamFrom, "stream-from", "", "Stage to stream invocations from; runs local server so you can point the provider (e.g. Lambda event source) at this process via a tunnel")
	cmd.Flags().StringVar(&tunnelURL, "tunnel-url", "", "Public URL of your tunnel (e.g. from ngrok); show in instructions as the invocation target")
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
