package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/recovery"
	"github.com/spf13/cobra"
)

func newRecoverCmd(opts *GlobalOptions) *cobra.Command {
	var mode string

	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover from an unfinished transaction journal",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Recover(opts.ConfigPath, opts.Stage, recovery.Mode(mode))
			if err != nil {
				return printFailure("recover", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("recover", result)
			}
			return printSuccess("recover", result)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", string(recovery.ModeRollback), "Recovery mode: rollback|resume|inspect")
	return cmd
}
