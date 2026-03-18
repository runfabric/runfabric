package cli

import (
	"fmt"
	"os"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/configpatch"
	"github.com/runfabric/runfabric/engine/internal/extensions/runtime"
	"github.com/spf13/cobra"
)

func newAiCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI workflow utilities",
		Long:  "AI workflow utilities (Phase 14): validate and graph export. These are introspection tools, not a parallel lifecycle.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAiValidateCmd(opts), newAiGraphCmd(opts))
	return cmd
}

func newAiValidateCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate aiWorkflow config and graph",
		Long:  "Validates runfabric.yml and, when aiWorkflow.enable is true, validates nodes/edges/types/entrypoint.",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			cfgPath, err := configpatch.ResolveConfigPath(opts.ConfigPath, cwd, 5)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai validate", nil, &runtime.ErrorResponse{Code: "config_not_found", Message: err.Error()})
				}
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai validate", nil, &runtime.ErrorResponse{Code: "config_load_failed", Message: err.Error()})
				}
				return err
			}
			if err := config.Validate(cfg); err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai validate", nil, &runtime.ErrorResponse{Code: "invalid_config", Message: err.Error()})
				}
				return err
			}

			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "ai validate", map[string]any{
					"config":  cfgPath,
					"enabled": cfg.AiWorkflow != nil && cfg.AiWorkflow.Enable,
					"entry": func() string {
						if cfg.AiWorkflow != nil {
							return cfg.AiWorkflow.Entrypoint
						}
						return ""
					}(),
					"nodes": func() int {
						if cfg.AiWorkflow != nil {
							return len(cfg.AiWorkflow.Nodes)
						}
						return 0
					}(),
					"edges": func() int {
						if cfg.AiWorkflow != nil {
							return len(cfg.AiWorkflow.Edges)
						}
						return 0
					}(),
					"validated": true,
				}, nil)
			}

			if cfg.AiWorkflow == nil || !cfg.AiWorkflow.Enable {
				fmt.Fprintln(c.OutOrStdout(), "ai validate: ok (aiWorkflow disabled)")
				return nil
			}
			fmt.Fprintf(c.OutOrStdout(), "ai validate: ok (nodes=%d edges=%d entrypoint=%s)\n", len(cfg.AiWorkflow.Nodes), len(cfg.AiWorkflow.Edges), cfg.AiWorkflow.Entrypoint)
			return nil
		},
	}
	return cmd
}

func newAiGraphCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Compile and export the AI workflow graph",
		Long:  "Compiles the aiWorkflow DAG (topological order + execution levels) and prints a deterministic representation for tooling.",
		RunE: func(c *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			cfgPath, err := configpatch.ResolveConfigPath(opts.ConfigPath, cwd, 5)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai graph", nil, &runtime.ErrorResponse{Code: "config_not_found", Message: err.Error()})
				}
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai graph", nil, &runtime.ErrorResponse{Code: "config_load_failed", Message: err.Error()})
				}
				return err
			}
			if err := config.Validate(cfg); err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai graph", nil, &runtime.ErrorResponse{Code: "invalid_config", Message: err.Error()})
				}
				return err
			}
			if cfg.AiWorkflow == nil || !cfg.AiWorkflow.Enable {
				err := fmt.Errorf("aiWorkflow is not enabled (set aiWorkflow.enable: true)")
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai graph", nil, &runtime.ErrorResponse{Code: "ai_workflow_disabled", Message: err.Error()})
				}
				return err
			}

			g, err := config.CompileAiWorkflow(cfg.AiWorkflow)
			if err != nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai graph", nil, &runtime.ErrorResponse{Code: "compile_failed", Message: err.Error()})
				}
				return err
			}
			if g == nil {
				err := fmt.Errorf("no compiled graph produced")
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), false, "ai graph", nil, &runtime.ErrorResponse{Code: "compile_failed", Message: err.Error()})
				}
				return err
			}

			result := map[string]any{
				"config":     cfgPath,
				"entrypoint": g.Entrypoint,
				"hash":       g.Hash,
				"order":      g.Order,
				"levels":     g.Levels,
				"edges":      g.Edges,
			}
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "ai graph", result, nil)
			}
			fmt.Fprintf(c.OutOrStdout(), "ai graph: entrypoint=%s hash=%s\n", g.Entrypoint, g.Hash)
			fmt.Fprintf(c.OutOrStdout(), "order: %v\n", g.Order)
			fmt.Fprintf(c.OutOrStdout(), "levels: %v\n", g.Levels)
			return nil
		},
	}
	return cmd
}
