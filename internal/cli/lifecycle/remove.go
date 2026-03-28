package lifecycle

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newRemoveCmd(opts *common.GlobalOptions) *cobra.Command {
	var providerOverride string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove the deployed service",
		RunE: func(cmd *cobra.Command, args []string) error {
			service := resolveAppService(opts)
			common.StatusRunning(opts.JSONOutput, "Removing deployed resources...")
			result, err := service.Remove(opts.ConfigPath, opts.Stage, providerOverride)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Remove failed.")
				return common.PrintFailure("remove", err)
			}
			common.StatusDone(opts.JSONOutput, "Remove complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("remove", result)
			}
			return common.PrintSuccess("remove", result)
		},
	}
	cmd.Flags().StringVar(&providerOverride, "provider", "", "Provider key from providerOverrides (multi-cloud); e.g. aws, gcp")
	return cmd
}
