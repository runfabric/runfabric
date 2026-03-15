package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDocsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "docs command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
