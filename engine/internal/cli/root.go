package cli

import (
	"github.com/runfabric/runfabric/engine/internal/runtime"
	"github.com/spf13/cobra"
)

type GlobalOptions struct {
	ConfigPath string
	Stage      string
	JSONOutput bool
}

func NewRootCmd() *cobra.Command {
	opts := &GlobalOptions{}

	cmd := &cobra.Command{
		Use:     "runfabric",
		Short:   "RunFabric CLI",
		Long:    "RunFabric is a multi-provider serverless deployment framework.",
		Version: runtime.Version,
	}

	cmd.PersistentFlags().StringVarP(&opts.ConfigPath, "config", "c", "runfabric.yml", "Path to runfabric.yml")
	cmd.PersistentFlags().StringVarP(&opts.Stage, "stage", "s", "dev", "Deployment stage")
	cmd.PersistentFlags().BoolVar(&opts.JSONOutput, "json", false, "Emit machine-readable JSON output")

	cmd.AddCommand(
		newDoctorCmd(opts),
		newPlanCmd(opts),
		newDeployCmd(opts),
		newDeployFunctionStandaloneCmd(opts),
		newRemoveCmd(opts),
		newLockStealCmd(opts),
		newInvokeCmd(opts),
		newLogsCmd(opts),
		newInspectCmd(opts),
		newRecoverCmd(opts),
		newRecoverDryRunCmd(opts),
		newUnlockCmd(opts),
		newBackendMigrateCmd(opts),
		newInitCmd(opts),
		newDocsCmd(opts),
		newBuildCmd(opts),
		newPackageCmd(opts),
		newMigrateCmd(opts),
		newCallLocalCmd(opts),
		newDevCmd(opts),
		newTracesCmd(opts),
		newMetricsCmd(opts),
		newAddonsCmd(opts),
		newProvidersCmd(opts),
		newPrimitivesCmd(opts),
		newComposeCmd(opts),
		newFabricCmd(opts),
		newConfigAPICmd(opts),
		newDashboardCmd(opts),
		newDaemonCmd(opts),
		newStateCmd(opts),
		newListCmd(opts),
		newReleasesCmd(opts),
		newTestCmd(opts),
		newDebugCmd(opts),
	)

	return cmd
}
