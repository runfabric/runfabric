// Package project groups project management commands: init, generate, list, inspect, compose
package project

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common
type GlobalOptions = common.GlobalOptions

// RegisterCommands returns all project commands for registration with the root command
func RegisterCommands(opts *GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newInitCmd(opts),
		newGenerateCmd(opts),
		newListCmd(opts),
		newInspectCmd(opts),
		newComposeCmd(opts),
		newTestCmd(opts),
	}
}
