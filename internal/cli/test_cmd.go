package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newTestCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Run the project test suite",
		Long:  "Runs tests (npm test, go test, or pytest) in the project directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Test(opts.ConfigPath)
			if err != nil {
				return printFailure("test", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("test", result)
			}
			return printSuccess("test", result)
		},
	}
}
