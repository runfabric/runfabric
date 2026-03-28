package invocation

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newTracesCmd(opts *common.GlobalOptions) *cobra.Command {
	var provider string
	var all bool
	var service string

	cmd := &cobra.Command{
		Use:   "traces",
		Short: "View traces for the deployed service",
		Long:  "View traces aggregated by service/stage. Use --all to request aggregation for all functions (when backend supports it).",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Fetching traces...")
			result, err := app.Traces(opts.ConfigPath, opts.Stage, provider, all, service)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Traces failed.")
				return common.PrintFailure("traces", err)
			}
			common.StatusDone(opts.JSONOutput, "Traces complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("traces", result)
			}
			return common.PrintSuccess("traces", result)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate traces for all functions (by service/stage)")
	cmd.Flags().StringVar(&service, "service", "", "Service name scope (must match runfabric.yml service when set)")
	return cmd
}
