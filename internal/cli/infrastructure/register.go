// Package infrastructure groups state and infrastructure commands: state, backend-migrate, lock, unlock
package infrastructure

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// RegisterCommands returns all infrastructure commands for registration with the root command
func RegisterCommands(opts *common.GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newStateCmd(opts),
	}
}
