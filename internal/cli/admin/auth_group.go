package admin

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// auth groups authentication operations while keeping existing top-level commands.
func newAuthGroupCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newLoginCmd(opts),
		newWhoAmICmd(opts),
		newLogoutCmd(opts),
		newAuthCmd(opts),
	)
	return cmd
}
