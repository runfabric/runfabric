package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/internal/cli/common"
	"github.com/runfabric/runfabric/platform/core/model/configpatch"
	"github.com/runfabric/runfabric/platform/workflow/app"
	coreapp "github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newPackageCmd(opts *common.GlobalOptions) *cobra.Command {
	var function, outDir string
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Package the service for deployment",
		Long:  "Produce deployment artifacts using the same build path as plan/deploy. Use --function to package a single function, --out for output directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			configPath, _, err := configpatch.ResolveConfigAndRoot(opts.ConfigPath, cwd, 5)
			if err != nil {
				return err
			}
			result, err := app.Build(configPath, coreapp.BuildOptions{
				NoCache:        true, // package produces fresh artifacts
				OutDir:         outDir,
				FunctionFilter: function,
			})
			if err != nil {
				return err
			}
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					fmt.Fprintf(cmd.OutOrStderr(), "package: %s\n", e)
				}
				if len(result.Artifacts) == 0 {
					return fmt.Errorf("package failed for requested function(s)")
				}
			}
			if opts.JSONOutput {
				out := map[string]any{
					"ok":        len(result.Errors) == 0,
					"command":   "package",
					"artifacts": result.Artifacts,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			for _, a := range result.Artifacts {
				fmt.Fprintf(cmd.OutOrStdout(), "package: %s -> %s\n", a.Function, a.OutputPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&function, "function", "f", "", "Package only this function")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for package artifacts")
	return cmd
}
