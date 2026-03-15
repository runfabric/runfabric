package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newDeployCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the service",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Deploy(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("deploy", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("deploy", result)
			}
			return printSuccess("deploy", result)
		},
	}
}
