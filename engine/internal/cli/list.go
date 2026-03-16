package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List functions and deployment status",
		Long:  "Lists functions from runfabric.yml and whether each is deployed (from receipt).",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Listing functions...")
			result, err := app.List(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "List failed.")
				return printFailure("list", err)
			}
			statusDone(opts.JSONOutput, "List complete.")
			if opts.JSONOutput {
				return printJSONSuccess("list", result)
			}
			return printSuccess("list", result)
		},
	}
}
