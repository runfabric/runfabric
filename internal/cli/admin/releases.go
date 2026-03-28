package admin

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newReleasesCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "releases",
		Short: "List deployment history (releases)",
		Long:  "Lists deployments (stages and timestamps) from the receipt backend. Same as runfabric deploy list.",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Listing releases...")
			result, err := app.Releases(opts.ConfigPath)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Releases failed.")
				return common.PrintFailure("releases", err)
			}
			common.StatusDone(opts.JSONOutput, "Releases complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("releases", result)
			}
			return common.PrintSuccess("releases", result)
		},
	}
}
