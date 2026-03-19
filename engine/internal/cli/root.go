package cli

import (
	"os"

	"github.com/runfabric/runfabric/engine/internal/extensions/runtime"
	"github.com/spf13/cobra"
)

type GlobalOptions struct {
	ConfigPath     string
	Stage          string
	JSONOutput     bool
	NonInteractive bool
	AssumeYes      bool
	AutoInstallExt bool
}

func NewRootCmd() *cobra.Command {
	opts := &GlobalOptions{}

	cmd := &cobra.Command{
		Use:     "runfabric",
		Short:   "RunFabric CLI",
		Long:    "RunFabric is a multi-provider serverless deployment framework.",
		Version: runtime.Version,
	}
	// Avoid duplicate "Error:" output. We print errors in cmd/runfabric/main.go.
	// Also avoid showing usage for runtime errors (e.g. network failures); we only show usage
	// for flag parsing errors via SetFlagErrorFunc below.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		// For invalid flags / args, show usage to help the user recover quickly.
		_ = cmd.Usage()
		return err
	})

	cmd.PersistentFlags().StringVarP(&opts.ConfigPath, "config", "c", "runfabric.yml", "Path to runfabric.yml")
	cmd.PersistentFlags().StringVarP(&opts.Stage, "stage", "s", "dev", "Deployment stage")
	cmd.PersistentFlags().BoolVar(&opts.JSONOutput, "json", false, "Emit machine-readable JSON output")
	cmd.PersistentFlags().BoolVar(&opts.NonInteractive, "non-interactive", false, "Disable interactive prompts (for CI/MCP)")
	cmd.PersistentFlags().BoolVarP(&opts.AssumeYes, "yes", "y", false, "Assume yes for any confirmation prompt")
	cmd.PersistentFlags().BoolVar(&opts.AutoInstallExt, "auto-install-extensions", false, "Auto-install missing external extensions from registry (prompts unless -y)")

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Plumb confirmation/CI behavior into app bootstrap without threading opts everywhere.
		// These env vars are process-local and only affect this invocation.
		if opts.NonInteractive {
			_ = os.Setenv("RUNFABRIC_NON_INTERACTIVE", "1")
		}
		if opts.AssumeYes {
			_ = os.Setenv("RUNFABRIC_ASSUME_YES", "1")
		}
		if opts.AutoInstallExt {
			_ = os.Setenv("RUNFABRIC_AUTO_INSTALL_EXTENSIONS", "1")
		}
	}

	cmd.AddCommand(
		newDoctorCmd(opts),
		newAiCmd(opts),
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
		newGenerateCmd(opts),
		newDocsCmd(opts),
		newBuildCmd(opts),
		newPackageCmd(opts),
		newMigrateCmd(opts),
		newCallLocalCmd(opts),
		newDevCmd(opts),
		newTracesCmd(opts),
		newMetricsCmd(opts),
		newAddonsCmd(opts),
		newPluginCmd(opts),
		newExtensionCmd(opts),
		newLoginCmd(opts),
		newWhoAmICmd(opts),
		newLogoutCmd(opts),
		newAuthCmd(opts),
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
