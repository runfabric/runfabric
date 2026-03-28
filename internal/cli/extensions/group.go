package extensions

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newExtensionsGroupCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extensions",
		Short: "Extension ecosystem commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newAddonsCmd(opts),
		newPluginCmd(opts),
		newExtensionCmd(opts),
		newExtensionPublishCmd(opts),
		newProvidersCmd(opts),
		common.NewPrimitivesCmd(opts),
	)
	return cmd
}
