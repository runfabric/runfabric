package invocation

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newMetricsCmd(opts *common.GlobalOptions) *cobra.Command {
	var provider string
	var all bool
	var service string

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "View metrics for the deployed service",
		Long:  "View metrics aggregated by service/stage. Use --all to request aggregation for all functions (when backend supports it).",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Fetching metrics...")
			result, err := app.Metrics(opts.ConfigPath, opts.Stage, provider, all, service)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Metrics failed.")
				return common.PrintFailure("metrics", err)
			}
			common.StatusDone(opts.JSONOutput, "Metrics complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("metrics", result)
			}
			return common.PrintSuccess("metrics", result)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate metrics for all functions (by service/stage)")
	cmd.Flags().StringVar(&service, "service", "", "Service name scope (must match runfabric.yml service when set)")
	return cmd
}
