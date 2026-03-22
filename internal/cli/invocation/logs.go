package invocation

import (
	"fmt"
	"github.com/runfabric/runfabric/internal/cli/common"

	"github.com/runfabric/runfabric/internal/app"
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
		Long:  "Read logs for one function (--function) or all functions (--all). Use --provider when runfabric.yml has providerOverrides (multi-cloud). Use --service to enforce service scope.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				function = ""
			}
			if function == "" && !all {
				return fmt.Errorf("either --function or --all is required")
			}
			common.StatusRunning(opts.JSONOutput, "Fetching logs...")
			result, err := app.Logs(opts.ConfigPath, opts.Stage, function, providerOverride, service)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Logs failed.")
				return common.PrintFailure("logs", err)
			}
			common.StatusDone(opts.JSONOutput, "Logs complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("logs", result)
			}
			return common.PrintSuccess("logs", result)
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name (omit with --all)")
	cmd.Flags().BoolVar(&all, "all", false, "Aggregate logs for all functions in the service/stage")
	cmd.Flags().StringVar(&service, "service", "", "Service name scope (must match runfabric.yml service when set)")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")

	return cmd
}
