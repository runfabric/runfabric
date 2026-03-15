package cli

import (
	"github.com/runfabric/runfabric/internal/app"
	"github.com/spf13/cobra"
)

func newLogsCmd(opts *GlobalOptions) *cobra.Command {
	var function string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Read function logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.Logs(opts.ConfigPath, opts.Stage, function)
			if err != nil {
				return printFailure("logs", err)
			}
			if opts.JSONOutput {
				return printJSONSuccess("logs", result)
			}
			return printSuccess("logs", result)
		},
	}

	cmd.Flags().StringVarP(&function, "function", "f", "", "Function name")
	_ = cmd.MarkFlagRequired("function")

	return cmd
}
