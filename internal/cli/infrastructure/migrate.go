package infrastructure

import (
	"fmt"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newMigrateCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "migrate command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
