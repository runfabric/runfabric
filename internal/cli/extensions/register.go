// Package extensions groups extension/plugin management commands: addons, extension, plugin
package extensions

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common
type GlobalOptions = common.GlobalOptions

// RegisterCommands returns all extension commands for registration with the root command
func RegisterCommands(opts *GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newExtensionsGroupCmd(opts),
	}
}
