package invocation

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

func newInvokeLocalCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := newCallLocalCmd(opts)
	cmd.Use = "local"
	cmd.Short = "Run the service locally"
	return cmd
}
