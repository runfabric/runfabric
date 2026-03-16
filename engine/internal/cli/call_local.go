package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newCallLocalCmd(opts *GlobalOptions) *cobra.Command {
	var serve, watch bool
	var host, port, provider, method, path, query, header, body, eventFile, entry string

	cmd := &cobra.Command{
		Use:   "call-local",
		Short: "Run the service locally",
		Long:  "Starts a local HTTP server to run your handlers. Use --serve to keep it running (attach a debugger to this process to debug).",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Starting local server...")
			result, err := app.CallLocal(opts.ConfigPath, opts.Stage, host, port, serve)
			if err != nil {
				statusFail(opts.JSONOutput, "Call-local failed.")
				return printFailure("call-local", err)
			}
			statusDone(opts.JSONOutput, "Local server ready.")
			_ = watch
			_ = provider
			_ = method
			_ = path
			_ = query
			_ = header
			_ = body
			_ = eventFile
			_ = entry
			if opts.JSONOutput {
				return printJSONSuccess("call-local", result)
			}
			return printSuccess("call-local", result)
		},
	}

	cmd.Flags().BoolVar(&serve, "serve", true, "Start local server and keep running (default: true)")
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch and reload on file changes")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host for local server")
	cmd.Flags().StringVar(&port, "port", "3000", "Port for local server")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider to emulate")
	cmd.Flags().StringVar(&method, "method", "GET", "HTTP method (GET, POST, ...)")
	cmd.Flags().StringVar(&path, "path", "", "Request path")
	cmd.Flags().StringVar(&query, "query", "", "Query string (k=v&k2=v2)")
	cmd.Flags().StringVar(&header, "header", "", "Header (k:v)")
	cmd.Flags().StringVar(&body, "body", "", "Request body")
	cmd.Flags().StringVar(&eventFile, "event", "", "Event payload file")
	cmd.Flags().StringVar(&entry, "entry", "", "Entry path override")
	return cmd
}
