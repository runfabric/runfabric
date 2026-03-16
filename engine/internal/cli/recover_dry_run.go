package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newRecoverDryRunCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "recover-dry-run",
		Short: "Inspect recovery feasibility without mutating state",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Checking recovery feasibility...")
			result, err := app.RecoverDryRun(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "Recover-dry-run failed.")
				return printFailure("recover-dry-run", err)
			}
			statusDone(opts.JSONOutput, "Recover-dry-run complete.")
			if opts.JSONOutput {
				return printJSONSuccess("recover-dry-run", result)
			}
			return printSuccess("recover-dry-run", result)
		},
	}
}
