// Package router groups router management commands: deploy, status, endpoints, routing, dns-sync
package router

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// RegisterCommands returns all router commands for registration with the root command
func RegisterCommands(opts *common.GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newRouteCmd(opts),
	}
}
