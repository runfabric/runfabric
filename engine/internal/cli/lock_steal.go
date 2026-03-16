package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newLockStealCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "lock-steal",
		Short: "Steal an expired remote lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Stealing lock...")
			result, err := app.LockSteal(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "Lock-steal failed.")
				return printFailure("lock-steal", err)
			}
			statusDone(opts.JSONOutput, "Lock-steal complete.")
			if opts.JSONOutput {
				return printJSONSuccess("lock-steal", result)
			}
			return printSuccess("lock-steal", result)
		},
	}
}
