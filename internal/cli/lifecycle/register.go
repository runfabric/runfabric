// Package lifecycle groups core workflow commands: doctor, plan, build, deploy, remove, recover
package lifecycle

import (
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/spf13/cobra"
)

// RegisterCommands returns all lifecycle commands for registration with the root command
func RegisterCommands(opts *common.GlobalOptions) []*cobra.Command {
	return []*cobra.Command{
		newDoctorCmd(opts),
		newPlanCmd(opts),
		newBuildCmd(opts),
		newPackageCmd(opts),
		newDeployCmd(opts),
		newDeployFunctionStandaloneCmd(opts),
		newRemoveCmd(opts),
		newRecoverCmd(opts),
		newRecoverDryRunCmd(opts),
	}
}
