package common

import (
	"os"

	"github.com/spf13/cobra"
)

// RootSpec configures common root command metadata for a binary.
type RootSpec struct {
	Use     string
	Short   string
	Long    string
	Version string
}

// NewBootstrappedRootCmd creates a root command with shared global flags and env plumbing.
func NewBootstrappedRootCmd(spec RootSpec, opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     spec.Use,
		Short:   spec.Short,
		Long:    spec.Long,
		Version: spec.Version,
	}

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
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

	return cmd
}
