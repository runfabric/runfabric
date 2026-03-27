package lifecycle

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newRecoverDryRunCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "recover-dry-run",
		Short: "Inspect recovery feasibility without mutating state",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Checking recovery feasibility...")
			result, err := app.RecoverDryRun(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Recover-dry-run failed.")
				return common.PrintFailure("recover-dry-run", err)
			}
			common.StatusDone(opts.JSONOutput, "Recover-dry-run complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("recover-dry-run", result)
			}
			return common.PrintSuccess("recover-dry-run", result)
		},
	}
}
