package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newRecoverDryRunCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "recover-dry-run",
		Short: "Inspect recovery feasibility without mutating state",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.RecoverDryRun(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("recover-dry-run", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("recover-dry-run", result)
			}
			return printSuccess("recover-dry-run", result)
		},
	}
}
