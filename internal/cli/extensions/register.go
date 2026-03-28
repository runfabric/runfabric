// Package extensions groups extension/plugin management commands: addons, extension, plugin
package extensions

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// RegisterCommands returns all extension commands for registration with the root command
func RegisterCommands(opts *common.GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newExtensionsGroupCmd(opts),
	}
}
