package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newMetricsCmd(opts *GlobalOptions) *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "View metrics for the deployed service",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Metrics(opts.ConfigPath, opts.Stage, provider)
			if err != nil {
				return printFailure("metrics", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("metrics", result)
			}
			return printSuccess("metrics", result)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (default: from config)")
	return cmd
}
