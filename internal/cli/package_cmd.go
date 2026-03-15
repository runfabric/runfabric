package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPackageCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "package command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
