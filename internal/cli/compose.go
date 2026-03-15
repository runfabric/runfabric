package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newComposeCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "compose command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
