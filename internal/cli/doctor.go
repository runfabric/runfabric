package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate config and provider readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.BackendDoctor(opts.ConfigPath, opts.Stage)
			if err != nil {
				return printFailure("doctor", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("doctor", result)
			}
			return printSuccess("doctor", result)
		},
	}
}
