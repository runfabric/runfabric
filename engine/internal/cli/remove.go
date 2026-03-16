package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newRemoveCmd(opts *GlobalOptions) *cobra.Command {
	var providerOverride string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove the deployed service",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Removing deployed resources...")
			result, err := app.Remove(opts.ConfigPath, opts.Stage, providerOverride)
			if err != nil {
				statusFail(opts.JSONOutput, "Remove failed.")
				return printFailure("remove", err)
			}
			statusDone(opts.JSONOutput, "Remove complete.")
			if opts.JSONOutput {
				return printJSONSuccess("remove", result)
			}
			return printSuccess("remove", result)
		},
	}
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	return cmd
}
