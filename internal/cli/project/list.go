package project

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List functions and deployment status",
		Long:  "Lists functions from runfabric.yml and whether each is deployed (from receipt).",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Listing functions...")
			result, err := app.List(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "List failed.")
				return common.PrintFailure("list", err)
			}
			common.StatusDone(opts.JSONOutput, "List complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("list", result)
			}
			return common.PrintSuccess("list", result)
		},
	}
}
