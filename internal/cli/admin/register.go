// Package admin groups administrative and system commands: auth, daemon, dashboard, debug, docs, releases
package admin

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// RegisterCommands returns all admin commands for registration with the root command
func RegisterCommands(opts *common.GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newAuthGroupCmd(opts),
		newDashboardCmd(opts),
		newDebugCmd(opts),
		newDocsCmd(opts),
		newReleasesCmd(opts),
	}
}
