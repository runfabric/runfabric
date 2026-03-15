package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newRemoveCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove the deployed service",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Remove(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("remove", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("remove", result)
			}
			return printSuccess("remove", result)
		},
	}
}
