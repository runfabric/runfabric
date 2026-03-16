package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newPlanCmd(opts *GlobalOptions) *cobra.Command {
	var providerOverride string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate a deployment plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Generating deployment plan...")
			result, err := app.Plan(opts.ConfigPath, opts.Stage, providerOverride)
			if err != nil {
				statusFail(opts.JSONOutput, "Plan failed.")
				return printFailure("plan", err)
			}
			statusDone(opts.JSONOutput, "Plan complete.")
			if opts.JSONOutput {
				return printJSONSuccess("plan", result)
			}
			return printSuccess("plan", result)
		},
	}
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	return cmd
}
