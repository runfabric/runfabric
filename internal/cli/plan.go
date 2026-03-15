package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newPlanCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Generate a deployment plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Plan(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("plan", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("plan", result)
			}
			return printSuccess("plan", result)
		},
	}
}
