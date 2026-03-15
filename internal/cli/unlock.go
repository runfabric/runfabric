package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newUnlockCmd(opts *GlobalOptions) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Remove a lock file manually",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Unlock(opts.ConfigPath, opts.Stage, force)
			if err != nil {
				return printFailure("unlock", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("unlock", result)
			}
			return printSuccess("unlock", result)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force unlock")
	return cmd
}
