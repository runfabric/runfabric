package cli

import (
	"os"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newInvokeCmd(opts *GlobalOptions) *cobra.Command {
	var function string
	var payload string

	cmd := &cobra.Command{
		Use:   "invoke",
		Short: "Invoke a deployed function",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Invoke(opts.ConfigPath, opts.Stage, function, []byte(payload))
			if err != nil {
				return printFailure("invoke", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("invoke", result)
			}
			return printSuccess("invoke", result)
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name")
	cmd.Flags().StringVarP(&payload, "payload", "p", "", "Inline JSON/text payload")
	_ = cmd.MarkFlagRequired("function")

	cmd.SetIn(os.Stdin)
	return cmd
}
