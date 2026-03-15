package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List functions and deployment status",
		Long:  "Lists functions from runfabric.yml and whether each is deployed (from receipt).",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.List(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("list", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("list", result)
			}
			return printSuccess("list", result)
		},
	}
}
