package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newCallLocalCmd(opts *GlobalOptions) *cobra.Command {
	var serve bool
	var host, port string

	cmd := &cobra.Command{
		Use:   "call-local",
		Short: "Run the service locally",
		Long:  "Starts a local HTTP server to run your handlers. Use --serve to keep it running (attach a debugger to this process to debug).",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.CallLocal(opts.ConfigPath, opts.Stage, host, port, serve)
			if err != nil {
				return printFailure("call-local", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("call-local", result)
			}
			return printSuccess("call-local", result)
		},
	}

	cmd.Flags().BoolVar(&serve, "serve", true, "Start local server and keep running (default: true)")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host for local server")
	cmd.Flags().StringVar(&port, "port", "3000", "Port for local server")
	return cmd
}
