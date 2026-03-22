// Package fabric groups fabric management commands: fabric
package fabric

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common
type GlobalOptions = common.GlobalOptions

// RegisterCommands returns all fabric commands for registration with the root command
func RegisterCommands(opts *GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newFabricCmd(opts),
	}
}
