package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newLockStealCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "lock-steal",
		Short: "Steal an expired remote lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.LockSteal(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("lock-steal", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("lock-steal", result)
			}
			return printSuccess("lock-steal", result)
		},
	}
}
