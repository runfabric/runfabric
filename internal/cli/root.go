package cli

import (
	"os"

	"github.com/runfabric/runfabric/internal/app"
	"github.com/runfabric/runfabric/internal/cli/admin"
	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/internal/cli/configuration"
	"github.com/runfabric/runfabric/internal/cli/extensions"
	"github.com/runfabric/runfabric/internal/cli/fabric"
	"github.com/runfabric/runfabric/internal/cli/infrastructure"
	"github.com/runfabric/runfabric/internal/cli/invocation"
	"github.com/runfabric/runfabric/internal/cli/lifecycle"
	"github.com/runfabric/runfabric/internal/cli/project"
	"github.com/runfabric/runfabric/platform/extensions/registry/loader/runtime"
	"github.com/spf13/cobra"
)

// GlobalOptions is re-exported from common for convenience
type GlobalOptions = common.GlobalOptions

func NewRootCmd() *cobra.Command {
	opts := &GlobalOptions{AppService: app.NewAppService()}

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

	var allCommands []*cobra.Command
	allCommands = append(allCommands, lifecycle.RegisterCommands(opts)...)
	allCommands = append(allCommands, invocation.RegisterCommands(opts)...)
	allCommands = append(allCommands, project.RegisterCommands(opts)...)
	allCommands = append(allCommands, configuration.RegisterCommands(opts)...)
	allCommands = append(allCommands, extensions.RegisterCommands(opts)...)
	allCommands = append(allCommands, infrastructure.RegisterCommands(opts)...)
	allCommands = append(allCommands, admin.RegisterCommands(opts)...)
	allCommands = append(allCommands, fabric.RegisterCommands(opts)...)
	// Common root-level commands
	allCommands = append(allCommands, common.NewWorkflowCmd(opts))

	cmd.AddCommand(allCommands...)

	return cmd
}
