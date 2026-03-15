package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newTracesCmd(opts *GlobalOptions) *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "traces",
		Short: "View traces for the deployed service",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Traces(opts.ConfigPath, opts.Stage, provider)
			if err != nil {
				return printFailure("traces", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("traces", result)
			}
			return printSuccess("traces", result)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (default: from config)")
	return cmd
}
