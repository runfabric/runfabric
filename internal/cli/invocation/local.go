package invocation

import "github.com/spf13/cobra"

func newInvokeLocalCmd(opts *GlobalOptions) *cobra.Command {
	cmd := newCallLocalCmd(opts)
	cmd.Use = "local"
	cmd.Short = "Run the service locally"
	return cmd
}
