package lifecycle

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/recovery"
	"github.com/spf13/cobra"
)

func newRecoverCmd(opts *GlobalOptions) *cobra.Command {
	var mode string

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover from an unfinished transaction journal",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Recovering...")
			result, err := app.Recover(opts.ConfigPath, opts.Stage, recovery.Mode(mode))
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Recover failed.")
				return common.PrintFailure("recover", err)
			}
			common.StatusDone(opts.JSONOutput, "Recover complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("recover", result)
			}
			return common.PrintSuccess("recover", result)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", string(recovery.ModeRollback), "Recovery mode: rollback|resume|inspect")
	return cmd
}
