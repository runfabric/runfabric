package extensions

import (
	"fmt"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newProvidersCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "providers command",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Executing %s command\\n", cmd.Use)
		},
	}
	return cmd
}
