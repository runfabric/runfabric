package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newInspectCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Inspect lock, journal, and receipt state",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Inspect(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("inspect", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("inspect", result)
			}
			return printSuccess("inspect", result)
		},
	}
}
