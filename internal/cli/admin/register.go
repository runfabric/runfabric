// Package admin groups administrative and system commands: auth, daemon, dashboard, debug, docs, releases
package admin

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common
type GlobalOptions = common.GlobalOptions

// RegisterCommands returns all admin commands for registration with the root command
func RegisterCommands(opts *GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newAuthGroupCmd(opts),
		newDaemonCmd(opts),
		newDashboardCmd(opts),
		newDebugCmd(opts),
		newDocsCmd(opts),
		newReleasesCmd(opts),
	}
}
