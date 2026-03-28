package lifecycle

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate config and provider readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Checking config and provider readiness...")
			result, err := app.BackendDoctor(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Doctor failed.")
				return common.PrintFailure("doctor", err)
			}
			common.StatusDone(opts.JSONOutput, "Doctor complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("doctor", result)
			}
			return common.PrintSuccess("doctor", result)
		},
	}
}
