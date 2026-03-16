package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newTestCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Run the project test suite",
		Long:  "Runs tests (npm test, go test, or pytest) in the project directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Running tests...")
			result, err := app.Test(opts.ConfigPath)
			if err != nil {
				statusFail(opts.JSONOutput, "Test failed.")
				return printFailure("test", err)
			}
			statusDone(opts.JSONOutput, "Test complete.")
			if opts.JSONOutput {
				return printJSONSuccess("test", result)
			}
			return printSuccess("test", result)
		},
	}
}
