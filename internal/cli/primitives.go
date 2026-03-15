package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPrimitivesCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "primitives",
		Short: "primitives command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
