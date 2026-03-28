package project

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newTestCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Run the project test suite",
		Long:  "Runs tests (npm test, go test, or pytest) in the project directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Running tests...")
			result, err := app.Test(opts.ConfigPath)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Test failed.")
				return common.PrintFailure("test", err)
			}
			common.StatusDone(opts.JSONOutput, "Test complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("test", result)
			}
			return common.PrintSuccess("test", result)
		},
	}
}
