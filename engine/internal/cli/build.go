package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/engine/internal/app"
	"github.com/runfabric/runfabric/engine/internal/configpatch"
	"github.com/spf13/cobra"
)

func newBuildCmd(opts *GlobalOptions) *cobra.Command {
	var outDir string
	var noCache bool
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the service",
		Long:  "Build artifacts for deployment. Uses the same build path as plan/deploy and per-function cache under .runfabric/cache; use --no-cache to force rebuild.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			configPath, _, err := configpatch.ResolveConfigAndRoot(opts.ConfigPath, cwd, 5)
			if err != nil {
				return err
			}
			result, err := app.Build(configPath, app.BuildOptions{
				NoCache: noCache,
				OutDir:  outDir,
			})
			if err != nil {
				return err
			}
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					fmt.Fprintf(cmd.OutOrStderr(), "build: %s\n", e)
				}
			}
			if opts.JSONOutput {
				out := map[string]any{
					"ok":        len(result.Errors) == 0,
					"command":   "build",
					"artifacts": result.Artifacts,
					"cacheHit":  result.CacheHit,
					"errors":    result.Errors,
				}
				if result.AiWorkflowHash != "" {
					out["aiWorkflowHash"] = result.AiWorkflowHash
					out["aiWorkflowEntrypoint"] = result.AiWorkflowEntrypoint
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if result.AiWorkflowHash != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "build: aiWorkflow hash=%s entrypoint=%s\n", result.AiWorkflowHash, result.AiWorkflowEntrypoint)
			}
			for _, a := range result.Artifacts {
				label := "built"
				for _, hit := range result.CacheHit {
					if hit == a.Function {
						label = "cache hit"
						break
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "build: %s %s -> %s\n", a.Function, label, a.OutputPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory for build artifacts")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Ignore cache and force rebuild")
	return cmd
}
