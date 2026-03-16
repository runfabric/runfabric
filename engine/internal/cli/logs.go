package cli

import (
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newLogsCmd(opts *GlobalOptions) *cobra.Command {
	var function string
	var all bool
	var service string
	var providerOverride string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Read function logs",
		Long:  "Read logs for one function (--function) or all functions (--all). Use --provider when runfabric.yml has providerOverrides (multi-cloud). Use --service to scope by service name when aggregating.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				function = ""
			}
			if function == "" && !all {
				return fmt.Errorf("either --function or --all is required")
			}
			statusRunning(opts.JSONOutput, "Fetching logs...")
			result, err := app.Logs(opts.ConfigPath, opts.Stage, function, providerOverride)
			if err != nil {
				statusFail(opts.JSONOutput, "Logs failed.")
				return printFailure("logs", err)
			}
			if service != "" {
				_ = service // reserved for future compose / multi-service filtering
			}
			statusDone(opts.JSONOutput, "Logs complete.")
			if opts.JSONOutput {
				return printJSONSuccess("logs", result)
			}
			return printSuccess("logs", result)
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name (omit with --all)")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate logs for all functions in the service/stage")
	cmd.Flags().StringVar(&service, "service", "", "Service name (for future multi-service/compose scope)")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")

	return cmd
}
