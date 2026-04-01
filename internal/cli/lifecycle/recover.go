package lifecycle

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newRecoverCmd(opts *common.GlobalOptions) *cobra.Command {
	var mode string

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover from an unfinished transaction journal",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Recovering...")
			result, err := app.RecoverByMode(opts.ConfigPath, opts.Stage, mode)
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

	cmd.Flags().StringVar(&mode, "mode", "rollback", "Recovery mode: rollback|resume|inspect")
	return cmd
}
