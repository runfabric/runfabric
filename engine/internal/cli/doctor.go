package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate config and provider readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Checking config and provider readiness...")
			result, err := app.BackendDoctor(opts.ConfigPath, opts.Stage)
			if err != nil {
				statusFail(opts.JSONOutput, "Doctor failed.")
				return printFailure("doctor", err)
			}
			statusDone(opts.JSONOutput, "Doctor complete.")
			if opts.JSONOutput {
				return printJSONSuccess("doctor", result)
			}
			return printSuccess("doctor", result)
		},
	}
}
