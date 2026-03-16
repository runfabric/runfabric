package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newMetricsCmd(opts *GlobalOptions) *cobra.Command {
	var provider string
	var all bool
	var service string

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "View metrics for the deployed service",
		Long:  "View metrics aggregated by service/stage. Use --all to request aggregation for all functions (when backend supports it).",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Fetching metrics...")
			result, err := app.Metrics(opts.ConfigPath, opts.Stage, provider, all)
			if err != nil {
				statusFail(opts.JSONOutput, "Metrics failed.")
				return printFailure("metrics", err)
			}
			if service != "" {
				_ = service // reserved for future multi-service scope
			}
			statusDone(opts.JSONOutput, "Metrics complete.")
			if opts.JSONOutput {
				return printJSONSuccess("metrics", result)
			}
			return printSuccess("metrics", result)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate metrics for all functions (by service/stage)")
	cmd.Flags().StringVar(&service, "service", "", "Service name (for future multi-service scope)")
	return cmd
}
