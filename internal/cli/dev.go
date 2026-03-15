package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newDevCmd(opts *GlobalOptions) *cobra.Command {
	var host, port string
	var watch bool

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Local dev with optional watch",
		Long:  "Runs your service locally (same as call-local --serve). Use for local testing and debugging.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.CallLocal(opts.ConfigPath, opts.Stage, host, port, true)
			if err != nil {
				return printFailure("dev", err)
			}
			// Planned: file watch + rebuild (AGENTS lifecycle: build|package).
			_ = watch
			if opts.JSONOutput {
				return printJSONSuccess("dev", result)
			}
			return printSuccess("dev", result)
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host for local server")
	cmd.Flags().StringVar(&port, "port", "3000", "Port for local server")
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch files and rebuild (planned)")
	return cmd
}
