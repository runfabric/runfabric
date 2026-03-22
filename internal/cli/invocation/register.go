// Package invocation groups function invocation and observation commands: invoke, logs, traces, metrics, call-local, dev
package invocation

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common
type GlobalOptions = common.GlobalOptions

// RegisterCommands returns all invocation commands for registration with the root command
func RegisterCommands(opts *GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newInvokeCmd(opts),
	}
}
