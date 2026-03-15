package cli

import (
	"github.com/runfabric/runfabric/internal/runtime"
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
		Use:   "runfabric",
		Short: "RunFabric CLI",
		Long:  "RunFabric is a multi-provider serverless deployment framework.",
		Version: runtime.Version,
	}

	cmd.PersistentFlags().StringVarP(&opts.ConfigPath, "config", "c", "runfabric.yml", "Path to runfabric.yml")
	cmd.PersistentFlags().StringVarP(&opts.Stage, "stage", "s", "dev", "Deployment stage")
	cmd.PersistentFlags().BoolVar(&opts.JSONOutput, "json", false, "Emit machine-readable JSON output")

	cmd.AddCommand(
		newDoctorCmd(opts),
		newPlanCmd(opts),
		newDeployCmd(opts),
		newRemoveCmd(opts),
		newLockStealCmd(opts),
		newInvokeCmd(opts),
		newLogsCmd(opts),
		newInspectCmd(opts),
		newRecoverCmd(opts),
		newUnlockCmd(opts),
		newInitCmd(opts),
		newDocsCmd(opts),
		newBuildCmd(opts),
		newPackageCmd(opts),
		newMigrateCmd(opts),
		newCallLocalCmd(opts),
		newDevCmd(opts),
		newTracesCmd(opts),
		newMetricsCmd(opts),
		newProvidersCmd(opts),
		newPrimitivesCmd(opts),
		newComposeCmd(opts),
		newStateCmd(opts),
		newListCmd(opts),
		newTestCmd(opts),
		newDebugCmd(opts),
	)

	return cmd
}
