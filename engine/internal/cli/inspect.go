package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newInspectCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Inspect lock, journal, and receipt state",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Inspecting state...")
			result, err := app.Inspect(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "Inspect failed.")
				return printFailure("inspect", err)
			}
			statusDone(opts.JSONOutput, "Inspect complete.")
			if opts.JSONOutput {
				return printJSONSuccess("inspect", result)
			}
			return printSuccess("inspect", result)
		},
	}
}
