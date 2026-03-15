package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBuildCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "build command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
