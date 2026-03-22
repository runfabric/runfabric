package fabric

import (
	"encoding/json"
	"fmt"

	"github.com/runfabric/runfabric/internal/cli/common"

	"github.com/runfabric/runfabric/internal/app"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/spf13/cobra"
)

func newFabricCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fabric",
		Short: "Runtime fabric: active-active deploy, health check, endpoints",
		Long:  "When runfabric.yml has fabric.targets (provider keys from providerOverrides), deploy to multiple targets and run health checks. Use fabric deploy for active-active; fabric status to check endpoint health; fabric endpoints to list URLs (e.g. for failover/latency routing).",
	}
	cmd.AddCommand(newFabricDeployCmd(opts), newFabricStatusCmd(opts), newFabricEndpointsCmd(opts), newFabricRoutingCmd(opts))
	return cmd
}

func newFabricDeployCmd(opts *GlobalOptions) *cobra.Command {
	var rollbackOnFailure, noRollbackOnFailure bool
	c := &cobra.Command{
		Use:   "deploy",
		Short: "Active-active deploy to all fabric targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Fabric deploy...")
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Fabric deploy failed.")
				return common.PrintFailure("fabric deploy", err)
			}
			targets := app.FabricTargets(ctx.Config)
			if len(targets) == 0 {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No fabric targets; add fabric.targets (provider keys) and providerOverrides to runfabric.yml.\n")
				}
				return nil
			}
			fabricState, err := app.FabricDeploy(opts.ConfigPath, opts.Stage, rollbackOnFailure, noRollbackOnFailure)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Fabric deploy failed.")
				return common.PrintFailure("fabric deploy", err)
			}
			common.StatusDone(opts.JSONOutput, "Fabric deploy complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("fabric deploy", fabricState)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deployed to %d target(s): %v\n", len(fabricState.Endpoints), targets)
			for _, e := range fabricState.Endpoints {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", e.Provider, e.URL)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	c.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")
	return c
}

func newFabricStatusCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check health of fabric endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Fabric status...")
			fabricState, err := app.FabricHealth(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Fabric status failed.")
				return common.PrintFailure("fabric status", err)
			}
			if fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No fabric state; run 'runfabric fabric deploy' first.\n")
				}
				return nil
			}
			common.StatusDone(opts.JSONOutput, "Fabric status complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("fabric status", fabricState)
			}
			for _, e := range fabricState.Endpoints {
				healthy := "?"
				if e.Healthy != nil {
					if *e.Healthy {
						healthy = "ok"
					} else {
						healthy = "fail"
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s [%s]\n", e.Provider, e.URL, healthy)
			}
			return nil
		},
	}
}

func newFabricEndpointsCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "endpoints",
		Short: "List fabric endpoints (for failover/latency routing)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("fabric endpoints", err)
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No fabric state; run 'runfabric fabric deploy' first.\n")
				}
				return nil
			}
			if opts.JSONOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(fabricState)
			}
			for _, e := range fabricState.Endpoints {
				fmt.Fprintln(cmd.OutOrStdout(), e.URL)
			}
			return nil
		},
	}
}

func newFabricRoutingCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "routing",
		Short: "Generate DNS/LB configuration hints based on fabric routing strategy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("fabric routing", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No fabric routing strategy configured. Set 'routing: failover|latency|round-robin' in fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No fabric state; run 'runfabric fabric deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateFabricRoutingConfig(fabricState, ctx.Config)
			if opts.JSONOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(routingCfg)
			}
			guide := app.FormatFabricRoutingGuide(routingCfg)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", guide)
			return nil
		},
	}
}
