package cli

import (
	"github.com/spf13/cobra"
)

func newBuildCmd(opts *GlobalOptions) *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the service",
		Long:  "Build artifacts for deployment. Use -c for config, --stage for stage, --out for output directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = outDir
			stubMsg("build", "config", opts.ConfigPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for build artifacts")
	return cmd
}
