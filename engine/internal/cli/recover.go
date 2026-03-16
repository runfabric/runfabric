package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/runfabric/runfabric/engine/internal/recovery"
	"github.com/spf13/cobra"
)

func newRecoverCmd(opts *GlobalOptions) *cobra.Command {
	var mode string

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover from an unfinished transaction journal",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Recovering...")
			result, err := app.Recover(opts.ConfigPath, opts.Stage, recovery.Mode(mode))
			if err != nil {
				statusFail(opts.JSONOutput, "Recover failed.")
				return printFailure("recover", err)
			}
			statusDone(opts.JSONOutput, "Recover complete.")
			if opts.JSONOutput {
				return printJSONSuccess("recover", result)
			}
			return printSuccess("recover", result)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", string(recovery.ModeRollback), "Recovery mode: rollback|resume|inspect")
	return cmd
}
