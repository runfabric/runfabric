package infrastructure

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newUnlockCmd(opts *common.GlobalOptions) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Remove a lock file manually",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Unlocking...")
			result, err := app.Unlock(opts.ConfigPath, opts.Stage, force)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Unlock failed.")
				return common.PrintFailure("unlock", err)
			}
			common.StatusDone(opts.JSONOutput, "Unlock complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("unlock", result)
			}
			return common.PrintSuccess("unlock", result)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force unlock")
	return cmd
}
