package cli

import (
	"os"

	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/spf13/cobra"
)

func newInvokeCmd(opts *GlobalOptions) *cobra.Command {
	var function string
	var payload string
	var providerOverride string

	cmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invoke a deployed function",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusRunning(opts.JSONOutput, "Invoking function...")
			result, err := app.Invoke(opts.ConfigPath, opts.Stage, function, providerOverride, []byte(payload))
			if err != nil {
				statusFail(opts.JSONOutput, "Invoke failed.")
				return printFailure("invoke", err)
			}
			statusDone(opts.JSONOutput, "Invoke complete.")
			if opts.JSONOutput {
				return printJSONSuccess("invoke", result)
			}
			return printSuccess("invoke", result)
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name")
	cmd.Flags().StringVarP(&payload, "payload", "p", "", "Inline JSON/text payload")
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	_ = cmd.MarkFlagRequired("function")

	cmd.SetIn(os.Stdin)
	return cmd
}
