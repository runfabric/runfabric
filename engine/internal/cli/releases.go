package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newReleasesCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "releases",
		Short: "List deployment history (releases)",
		Long:  "Lists deployments (stages and timestamps) from the receipt backend. Same as runfabric deploy list.",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Listing releases...")
			result, err := app.Releases(opts.ConfigPath)
			if err != nil {
				statusFail(opts.JSONOutput, "Releases failed.")
				return printFailure("releases", err)
			}
			statusDone(opts.JSONOutput, "Releases complete.")
			if opts.JSONOutput {
				return printJSONSuccess("releases", result)
			}
			return printSuccess("releases", result)
		},
	}
}
