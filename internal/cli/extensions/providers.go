package extensions

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newProvidersCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "providers command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
