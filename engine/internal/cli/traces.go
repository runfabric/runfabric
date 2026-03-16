package cli

import (
	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newTracesCmd(opts *GlobalOptions) *cobra.Command {
	var provider string
	var all bool
	var service string

	cmd := &cobra.Command{
		Use:   "traces",
		Short: "View traces for the deployed service",
		Long:  "View traces aggregated by service/stage. Use --all to request aggregation for all functions (when backend supports it).",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Fetching traces...")
			result, err := app.Traces(opts.ConfigPath, opts.Stage, provider, all)
			if err != nil {
				statusFail(opts.JSONOutput, "Traces failed.")
				return printFailure("traces", err)
			}
			if service != "" {
				_ = service // reserved for future multi-service scope
			}
			statusDone(opts.JSONOutput, "Traces complete.")
			if opts.JSONOutput {
				return printJSONSuccess("traces", result)
			}
			return printSuccess("traces", result)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate traces for all functions (by service/stage)")
	cmd.Flags().StringVar(&service, "service", "", "Service name (for future multi-service scope)")
	return cmd
}
