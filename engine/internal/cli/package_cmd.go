package cli

import (
	"github.com/spf13/cobra"
)

func newPackageCmd(opts *GlobalOptions) *cobra.Command {
	var function, outDir string
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Package the service for deployment",
		Long:  "Produce deployment artifacts. Use --function to package a single function, --out for output directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = function
			_ = outDir
			stubMsg("package", "config", opts.ConfigPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&function, "function", "f", "", "Package only this function")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for package artifacts")
	return cmd
}
