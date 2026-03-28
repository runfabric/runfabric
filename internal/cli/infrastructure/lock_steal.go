package infrastructure

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newLockStealCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "lock-steal",
		Short: "Steal an expired remote lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Stealing lock...")
			result, err := app.LockSteal(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Lock-steal failed.")
				return common.PrintFailure("lock-steal", err)
			}
			common.StatusDone(opts.JSONOutput, "Lock-steal complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("lock-steal", result)
			}
			return common.PrintSuccess("lock-steal", result)
		},
	}
}
