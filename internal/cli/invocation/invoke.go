package invocation

import (
	"os"

	"github.com/runfabric/runfabric/internal/cli/common"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func invokeRunCommand(opts *GlobalOptions, use string) *cobra.Command {
	var function string
	var payload string
	var providerOverride string

	cmd := &cobra.Command{
		Use:   use,
		Short: "Invoke a deployed function",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Invoking function...")
			result, err := app.Invoke(opts.ConfigPath, opts.Stage, function, providerOverride, []byte(payload))
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Invoke failed.")
				return common.PrintFailure("invoke", err)
			}
			common.StatusDone(opts.JSONOutput, "Invoke complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("invoke", result)
			}
			return common.PrintSuccess("invoke", result)
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name")
	cmd.Flags().StringVarP(&payload, "payload", "p", "", "Inline JSON/text payload")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	_ = cmd.MarkFlagRequired("function")

	cmd.SetIn(os.Stdin)
	return cmd
}

func newInvokeCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invocation and local execution commands",
		Long:  "Grouped invocation namespace. Use subcommands like invoke run, invoke logs, invoke traces, invoke metrics, invoke local, invoke dev.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		invokeRunCommand(opts, "run"),
		newLogsCmd(opts),
		newTracesCmd(opts),
		newMetricsCmd(opts),
		newInvokeLocalCmd(opts),
		newDevCmd(opts),
	)
	return cmd
}
