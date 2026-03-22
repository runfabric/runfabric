package common

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
	"github.com/spf13/cobra"
)

func NewPrimitivesCmd(opts *GlobalOptions) *cobra.Command {
	var provider string
	var kind string
	cmd := &cobra.Command{
		Use:   "primitives",
		Short: "Show available trigger/resource/workflow primitives",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := map[string]any{}

			switch strings.TrimSpace(kind) {
			case "", "all":
				out["triggers"] = primitivesTriggers(provider)
				out["resources"] = primitivesResources()
				out["workflows"] = primitivesWorkflows()
			case "triggers":
				out["triggers"] = primitivesTriggers(provider)
			case "resources":
				out["resources"] = primitivesResources()
			case "workflows":
				out["workflows"] = primitivesWorkflows()
			default:
				return fmt.Errorf("--kind must be triggers, resources, workflows, or all")
			}

			if opts.JSONOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			if v, ok := out["triggers"]; ok {
				fmt.Fprintln(cmd.OutOrStdout(), "Triggers:")
				switch t := v.(type) {
				case []string:
					fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", strings.Join(t, ", "))
				case map[string][]string:
					keys := make([]string, 0, len(t))
					for k := range t {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", k, strings.Join(t[k], ", "))
					}
				}
			}
			if v, ok := out["resources"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Resources:\n- %s\n", strings.Join(v.([]string), ", "))
			}
			if v, ok := out["workflows"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Workflows:\n- %s\n", strings.Join(v.([]string), ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Show trigger primitives for one provider ID")
	cmd.Flags().StringVar(&kind, "kind", "all", "Primitive category: triggers, resources, workflows, or all")
	return cmd
}

func primitivesTriggers(provider string) any {
	provider = strings.TrimSpace(provider)
	if provider != "" {
		out := planner.SupportedTriggers(provider)
		sort.Strings(out)
		return out
	}
	out := make(map[string][]string, len(planner.ProviderCapabilities))
	for p := range planner.ProviderCapabilities {
		ts := planner.SupportedTriggers(p)
		sort.Strings(ts)
		out[p] = ts
	}
	return out
}

func primitivesResources() []string {
	return []string{
		"database",
		"cache",
		"queue",
		"topic",
		"bucket",
		"rds",
		"elasticache",
	}
}

func primitivesWorkflows() []string {
	return []string{
		"steps.function",
		"steps.next",
		"steps.retry.attempts",
		"steps.retry.backoffSeconds",
	}
}
