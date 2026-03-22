// Package infrastructure groups state and infrastructure commands: state, backend-migrate, lock, unlock
package infrastructure

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common
type GlobalOptions = common.GlobalOptions

// RegisterCommands returns all infrastructure commands for registration with the root command
func RegisterCommands(opts *GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newStateCmd(opts),
		newMigrateCmd(opts),
	}
}
