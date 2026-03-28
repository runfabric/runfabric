package project

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newInspectCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Inspect lock, journal, and receipt state",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Inspecting state...")
			result, err := app.Inspect(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Inspect failed.")
				return common.PrintFailure("inspect", err)
			}
			common.StatusDone(opts.JSONOutput, "Inspect complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("inspect", result)
			}
			return common.PrintSuccess("inspect", result)
		},
	}
}
