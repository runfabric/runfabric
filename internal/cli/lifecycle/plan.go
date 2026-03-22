package lifecycle

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newPlanCmd(opts *GlobalOptions) *cobra.Command {
	var providerOverride string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate a deployment plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			service := resolveAppService(opts)
			common.StatusRunning(opts.JSONOutput, "Generating deployment plan...")
			result, err := service.Plan(opts.ConfigPath, opts.Stage, providerOverride)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Plan failed.")
				return common.PrintFailure("plan", err)
			}
			common.StatusDone(opts.JSONOutput, "Plan complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("plan", result)
			}
			return common.PrintSuccess("plan", result)
		},
	}
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	return cmd
}
