package admin

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newDebugCmd(opts *GlobalOptions) *cobra.Command {
	var host, port string

	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Run locally and attach a debugger",
		Long:  "Starts the local server and prints PID so you can attach your debugger (e.g. VS Code, Delve) to this process.",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Starting debug server...")
			result, err := app.Debug(opts.ConfigPath, opts.Stage, host, port)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Debug failed.")
				return common.PrintFailure("debug", err)
			}
			common.StatusDone(opts.JSONOutput, "Debug server ready.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("debug", result)
			}
			return common.PrintSuccess("debug", result)
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host for local server")
	cmd.Flags().StringVar(&port, "port", "3000", "Port for local server")
	return cmd
}
