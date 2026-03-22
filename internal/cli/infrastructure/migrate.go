package infrastructure

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMigrateCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "migrate command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
