package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/workflow/app"
	"github.com/spf13/cobra"
)

func newRouteCmd(opts *common.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "router",
		Short: "Runtime router: active-active deploy, health check, endpoints",
		Long:  "When runfabric.yml has fabric.targets (provider keys from providerOverrides), deploy to multiple targets and run health checks. Use router deploy for active-active; router status to check endpoint health; router endpoints to list URLs (e.g. for failover/latency routing).",
	}
	cmd.AddCommand(
		newRouteDeployCmd(opts),
		newRouteStatusCmd(opts),
		newRouteEndpointsCmd(opts),
		newRouteRoutingCmd(opts),
		newRouteSimulateCmd(opts),
		newRouteChaosVerifyCmd(opts),
		newRouteDNSSyncCmd(opts),
		newRouteDNSShiftCmd(opts),
		newRouteDNSReconcileCmd(opts),
		newRouteDNSRestoreCmd(opts),
		newRouteDNSHistoryCmd(opts),
	)
	return cmd
}

func newRouteDeployCmd(opts *common.GlobalOptions) *cobra.Command {
	var rollbackOnFailure, noRollbackOnFailure bool
	var syncDNS, syncDryRun, allowProdSync bool
	var enforceStageRollout bool
	var zoneID, accountID string
	c := &cobra.Command{
		Use:   "deploy",
		Short: "Active-active deploy to all router targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Router deploy...")
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Router deploy failed.")
				return common.PrintFailure("router deploy", err)
			}
			targets := app.FabricTargets(ctx.Config)
			if len(targets) == 0 {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router targets; add fabric.targets (provider keys) and providerOverrides to runfabric.yml.\n")
				}
				return nil
			}
			fabricState, err := app.FabricDeploy(opts.ConfigPath, opts.Stage, rollbackOnFailure, noRollbackOnFailure)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Router deploy failed.")
				return common.PrintFailure("router deploy", err)
			}
			common.StatusDone(opts.JSONOutput, "Router deploy complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router deploy", fabricState)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deployed to %d target(s): %v\n", len(fabricState.Endpoints), targets)
			for _, e := range fabricState.Endpoints {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", e.Provider, e.URL)
			}

			policy := app.RouterDNSSyncPolicyForStage(ctx.Config, opts.Stage)
			runSync := syncDNS || policy.AutoApply
			if runSync {
				effectiveDryRun := policy.DryRun
				if cmd.Flags().Changed("sync-dns-dry-run") {
					effectiveDryRun = syncDryRun
				}
				effectiveAllowProdSync := policy.AllowProdSync
				if cmd.Flags().Changed("allow-prod-dns-sync") {
					effectiveAllowProdSync = allowProdSync
				}
				effectiveEnforceRollout := policy.EnforceStageRollout
				if cmd.Flags().Changed("enforce-dns-sync-stage-rollout") {
					effectiveEnforceRollout = enforceStageRollout
				}
				if err := enforceDNSSyncStageGateWithPolicy(opts.Stage, effectiveAllowProdSync, effectiveEnforceRollout, policy.ApprovalEnvByStage, policy.RequireReason, policy.ReasonEnv); err != nil {
					return common.PrintFailure("router deploy", err)
				}
				if !opts.JSONOutput {
					if !syncDNS && policy.AutoApply {
						fmt.Fprintf(cmd.OutOrStdout(), "Auto router DNS sync policy matched stage %q; running post-deploy sync.\n", opts.Stage)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Running post-deploy router DNS sync via plugin %q...\n", app.SelectedRouterPlugin(ctx.Config))
				}
				routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
				providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID, policy.ZoneIDEnv, policy.AccountIDEnv)
				result, err := runRouterDNSSyncWithPolicy(ctx, routingCfg, providerZoneID, providerAccountID, effectiveDryRun, policy, cmd.OutOrStdout())
				if err != nil {
					common.StatusFail(opts.JSONOutput, "Router DNS sync failed.")
					return common.PrintFailure("router deploy", err)
				}
				if opts.JSONOutput {
					_ = result
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&rollbackOnFailure, "rollback-on-failure", false, "Rollback on deploy failure")
	c.Flags().BoolVar(&noRollbackOnFailure, "no-rollback-on-failure", false, "Do not rollback on deploy failure")
	c.Flags().BoolVar(&syncDNS, "sync-dns", false, "Run router DNS/LB sync after successful router deploy")
	c.Flags().BoolVar(&syncDryRun, "sync-dns-dry-run", false, "Preview router DNS/LB sync changes without applying them")
	c.Flags().BoolVar(&allowProdSync, "allow-prod-dns-sync", false, "Required to allow router DNS/LB sync when --stage prod")
	c.Flags().BoolVar(&enforceStageRollout, "enforce-dns-sync-stage-rollout", false, "Enforce staged rollout policy for DNS sync (dev -> staging -> prod)")
	c.Flags().StringVar(&zoneID, "zone-id", "", "Router zone ID for post-deploy sync (overrides RUNFABRIC_ROUTER_ZONE_ID)")
	c.Flags().StringVar(&accountID, "account-id", "", "Router account ID for post-deploy sync (overrides RUNFABRIC_ROUTER_ACCOUNT_ID)")
	return c
}

func newRouteStatusCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check health of router endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			common.StatusRunning(opts.JSONOutput, "Router status...")
			fabricState, err := app.FabricHealth(opts.ConfigPath, opts.Stage)
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Router status failed.")
				return common.PrintFailure("router status", err)
			}
			if fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			common.StatusDone(opts.JSONOutput, "Router status complete.")
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router status", fabricState)
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

func newRouteEndpointsCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "endpoints",
		Short: "List router endpoints (for failover/latency routing)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router endpoints", err)
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
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

func newRouteRoutingCmd(opts *common.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "routing",
		Short: "Generate DNS/LB configuration hints based on router routing strategy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router routing", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router routing strategy configured. Set 'routing: failover|latency|round-robin' in the fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
			if opts.JSONOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(routingCfg)
			}
			guide := app.FormatRouterRoutingGuide(routingCfg)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", guide)
			return nil
		},
	}
}

func newRouteSimulateCmd(opts *common.GlobalOptions) *cobra.Command {
	var requests int
	var down []string
	c := &cobra.Command{
		Use:   "simulate",
		Short: "Simulate local routing decisions (no provider API calls)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router simulate", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router routing strategy configured. Set 'routing: failover|latency|round-robin' in the fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
			result := app.SimulateRouterRouting(routingCfg, requests, down)
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router simulate", result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Router simulation (%s): requests=%d\n", result.Strategy, result.Requests)
			if len(result.Down) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Simulated down providers: %s\n", strings.Join(result.Down, ", "))
			}
			if !result.Available {
				fmt.Fprintln(cmd.OutOrStdout(), "No available endpoints under current simulation conditions.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dominant target: %s\n", result.Selected)
			printDistribution(cmd.OutOrStdout(), result.Distribution)
			return nil
		},
	}
	c.Flags().IntVar(&requests, "requests", 200, "Number of synthetic requests to simulate")
	c.Flags().StringArrayVar(&down, "down", nil, "Provider(s) to treat as unavailable during simulation")
	return c
}

func newRouteChaosVerifyCmd(opts *common.GlobalOptions) *cobra.Command {
	var requests int
	c := &cobra.Command{
		Use:   "chaos-verify",
		Short: "Run one-endpoint-down and all-endpoints-down failover checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router chaos-verify", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router routing strategy configured. Set 'routing: failover|latency|round-robin' in the fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
			report := app.VerifyRouterFailover(routingCfg, requests)
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router chaos-verify", report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Chaos verification (%s): %s\n", report.Strategy, map[bool]string{true: "pass", false: "fail"}[report.Pass])
			for _, scenario := range report.Scenarios {
				status := "pass"
				if !scenario.Pass {
					status = "fail"
				}
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"  [%s] %s down=%s selected=%s available=%t\n",
					status,
					scenario.Scenario,
					strings.Join(scenario.Down, ","),
					scenario.Selected,
					scenario.Available,
				)
			}
			return nil
		},
	}
	c.Flags().IntVar(&requests, "requests", 200, "Number of synthetic requests per chaos scenario")
	return c
}

func newRouteDNSSyncCmd(opts *common.GlobalOptions) *cobra.Command {
	var dryRun bool
	var allowProdSync bool
	var enforceStageRollout bool
	var zoneID, accountID string
	c := &cobra.Command{
		Use:   "dns-sync",
		Short: "Sync router routing via configured router plugin (idempotent)",
		Long: `Apply the router routing contract via the configured router plugin.

Credentials:
	RUNFABRIC_ROUTER_API_TOKEN  — required, read from environment only (never a flag).
	RUNFABRIC_ROUTER_ZONE_ID    — required; override via --zone-id flag.
	RUNFABRIC_ROUTER_ACCOUNT_ID — optional; enables LB pool/monitor management; override via --account-id flag.

When RUNFABRIC_ROUTER_ACCOUNT_ID is absent, only a DNS CNAME record is managed (DNS-only mode).
When present, a health-check monitor, LB pool, and zone-level load balancer are also reconciled.

Use --dry-run to preview planned changes without modifying the provider.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router dns-sync", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router routing strategy configured. Set 'routing: failover|latency|round-robin' in the fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
			policy := app.RouterDNSSyncPolicyForStage(ctx.Config, opts.Stage)
			effectiveDryRun := dryRun
			if !cmd.Flags().Changed("dry-run") && policy.DryRun {
				effectiveDryRun = true
			}
			effectiveAllowProdSync := policy.AllowProdSync
			if cmd.Flags().Changed("allow-prod-dns-sync") {
				effectiveAllowProdSync = allowProdSync
			}
			effectiveEnforceRollout := policy.EnforceStageRollout
			if cmd.Flags().Changed("enforce-dns-sync-stage-rollout") {
				effectiveEnforceRollout = enforceStageRollout
			}
			if err := enforceDNSSyncStageGateWithPolicy(opts.Stage, effectiveAllowProdSync, effectiveEnforceRollout, policy.ApprovalEnvByStage, policy.RequireReason, policy.ReasonEnv); err != nil {
				return common.PrintFailure("router dns-sync", err)
			}

			providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID, policy.ZoneIDEnv, policy.AccountIDEnv)

			result, err := runRouterDNSSyncWithPolicy(ctx, routingCfg, providerZoneID, providerAccountID, effectiveDryRun, policy, cmd.OutOrStdout())
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Router DNS sync failed.")
				return common.PrintFailure("router dns-sync", err)
			}
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router dns-sync", result)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned changes without applying them")
	c.Flags().BoolVar(&allowProdSync, "allow-prod-dns-sync", false, "Required to allow router DNS/LB sync when --stage prod")
	c.Flags().BoolVar(&enforceStageRollout, "enforce-dns-sync-stage-rollout", false, "Enforce staged rollout policy for DNS sync (dev -> staging -> prod)")
	c.Flags().StringVar(&zoneID, "zone-id", "", "Router zone ID (overrides RUNFABRIC_ROUTER_ZONE_ID)")
	c.Flags().StringVar(&accountID, "account-id", "", "Router account ID for LB operations (overrides RUNFABRIC_ROUTER_ACCOUNT_ID)")
	return c
}

func newRouteDNSShiftCmd(opts *common.GlobalOptions) *cobra.Command {
	var provider string
	var percent int
	var dryRun bool
	var allowProdSync bool
	var enforceStageRollout bool
	var zoneID, accountID string
	c := &cobra.Command{
		Use:   "dns-shift",
		Short: "Apply progressive canary traffic shift to one endpoint and sync DNS/LB",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router dns-shift", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router routing strategy configured. Set 'routing: failover|latency|round-robin' in the fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
			if !app.ApplyCanaryWeights(routingCfg, provider, percent) {
				return common.PrintFailure("router dns-shift", fmt.Errorf("canary provider %q not found in router endpoints", provider))
			}

			policy := app.RouterDNSSyncPolicyForStage(ctx.Config, opts.Stage)
			effectiveDryRun := dryRun
			if !cmd.Flags().Changed("dry-run") && policy.DryRun {
				effectiveDryRun = true
			}
			effectiveAllowProdSync := policy.AllowProdSync
			if cmd.Flags().Changed("allow-prod-dns-sync") {
				effectiveAllowProdSync = allowProdSync
			}
			effectiveEnforceRollout := policy.EnforceStageRollout
			if cmd.Flags().Changed("enforce-dns-sync-stage-rollout") {
				effectiveEnforceRollout = enforceStageRollout
			}
			if err := enforceDNSSyncStageGateWithPolicy(opts.Stage, effectiveAllowProdSync, effectiveEnforceRollout, policy.ApprovalEnvByStage, policy.RequireReason && !effectiveDryRun, policy.ReasonEnv); err != nil {
				return common.PrintFailure("router dns-shift", err)
			}
			providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID, policy.ZoneIDEnv, policy.AccountIDEnv)
			result, err := runRouterDNSSyncWithPolicy(ctx, routingCfg, providerZoneID, providerAccountID, effectiveDryRun, policy, cmd.OutOrStdout())
			if err != nil {
				return common.PrintFailure("router dns-shift", err)
			}
			payload := map[string]any{
				"provider": provider,
				"percent":  percent,
				"dryRun":   effectiveDryRun,
				"weights":  endpointWeightMap(routingCfg),
				"result":   result,
			}
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router dns-shift", payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Canary shift applied for %s (%d%% target weight)\n", provider, percent)
			printDistribution(cmd.OutOrStdout(), endpointWeightMap(routingCfg))
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "Provider endpoint name to receive canary traffic (required)")
	c.Flags().IntVar(&percent, "percent", 10, "Canary traffic percentage (1-99)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned canary shift without applying provider changes")
	c.Flags().BoolVar(&allowProdSync, "allow-prod-dns-sync", false, "Required to allow router DNS/LB sync when --stage prod")
	c.Flags().BoolVar(&enforceStageRollout, "enforce-dns-sync-stage-rollout", false, "Enforce staged rollout policy for DNS sync (dev -> staging -> prod)")
	c.Flags().StringVar(&zoneID, "zone-id", "", "Router zone ID (overrides RUNFABRIC_ROUTER_ZONE_ID)")
	c.Flags().StringVar(&accountID, "account-id", "", "Router account ID for LB operations (overrides RUNFABRIC_ROUTER_ACCOUNT_ID)")
	_ = c.MarkFlagRequired("provider")
	return c
}

func newRouteDNSReconcileCmd(opts *common.GlobalOptions) *cobra.Command {
	var apply bool
	var allowProdSync bool
	var enforceStageRollout bool
	var zoneID, accountID string
	c := &cobra.Command{
		Use:   "dns-reconcile",
		Short: "Report drift (dry-run) or reconcile router DNS/LB state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router dns-reconcile", err)
			}
			if ctx.Config.Fabric == nil || ctx.Config.Fabric.Routing == "" {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router routing strategy configured. Set 'routing: failover|latency|round-robin' in the fabric config.\n")
				}
				return nil
			}
			fabricState, err := state.LoadFabricState(ctx.RootDir, opts.Stage)
			if err != nil || fabricState == nil {
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "No router state; run 'runfabric router deploy' first.\n")
				}
				return nil
			}
			routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
			policy := app.RouterDNSSyncPolicyForStage(ctx.Config, opts.Stage)

			effectiveAllowProdSync := policy.AllowProdSync
			if cmd.Flags().Changed("allow-prod-dns-sync") {
				effectiveAllowProdSync = allowProdSync
			}
			effectiveEnforceRollout := policy.EnforceStageRollout
			if cmd.Flags().Changed("enforce-dns-sync-stage-rollout") {
				effectiveEnforceRollout = enforceStageRollout
			}
			if err := enforceDNSSyncStageGateWithPolicy(opts.Stage, effectiveAllowProdSync, effectiveEnforceRollout, policy.ApprovalEnvByStage, policy.RequireReason && apply, policy.ReasonEnv); err != nil {
				return common.PrintFailure("router dns-reconcile", err)
			}

			providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID, policy.ZoneIDEnv, policy.AccountIDEnv)
			dryRun := !apply
			result, err := runRouterDNSSyncWithPolicy(ctx, routingCfg, providerZoneID, providerAccountID, dryRun, policy, cmd.OutOrStdout())
			if err != nil {
				common.StatusFail(opts.JSONOutput, "Router DNS reconcile failed.")
				return common.PrintFailure("router dns-reconcile", err)
			}
			summary := app.RouterSyncSummaryFromResult(result)
			payload := map[string]any{
				"dryRun":  dryRun,
				"summary": summary,
				"result":  result,
			}
			historyAnalytics := loadRouterHistoryAnalytics(ctx.RootDir, opts.Stage, 5)
			if historyAnalytics != nil {
				payload["historyAnalytics"] = historyAnalytics
			}
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router dns-reconcile", payload)
			}
			if dryRun {
				if summary.DriftDetected {
					fmt.Fprintf(
						cmd.OutOrStdout(),
						"Drift detected: %d create, %d update, %d unchanged, %d delete-candidate.\n",
						summary.Create,
						summary.Update,
						summary.Noop,
						summary.DeleteCandidate,
					)
					printRouterResourceBreakdown(cmd.OutOrStdout(), summary)
					if historyAnalytics != nil {
						printRouterHistoryTrend(cmd.OutOrStdout(), *historyAnalytics)
					}
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No drift detected; desired and actual router state are aligned.\n")
					if historyAnalytics != nil {
						printRouterHistoryTrend(cmd.OutOrStdout(), *historyAnalytics)
					}
				}
				return nil
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Reconcile complete: %d created, %d updated, %d unchanged, %d delete-candidate.\n",
				summary.Create,
				summary.Update,
				summary.Noop,
				summary.DeleteCandidate,
			)
			printRouterResourceBreakdown(cmd.OutOrStdout(), summary)
			if historyAnalytics != nil {
				printRouterHistoryTrend(cmd.OutOrStdout(), *historyAnalytics)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&apply, "apply", false, "Apply reconcile changes (default is dry-run report only)")
	c.Flags().BoolVar(&allowProdSync, "allow-prod-dns-sync", false, "Required to allow router DNS/LB sync when --stage prod")
	c.Flags().BoolVar(&enforceStageRollout, "enforce-dns-sync-stage-rollout", false, "Enforce staged rollout policy for DNS sync (dev -> staging -> prod)")
	c.Flags().StringVar(&zoneID, "zone-id", "", "Router zone ID (overrides RUNFABRIC_ROUTER_ZONE_ID)")
	c.Flags().StringVar(&accountID, "account-id", "", "Router account ID for LB operations (overrides RUNFABRIC_ROUTER_ACCOUNT_ID)")
	return c
}

func newRouteDNSRestoreCmd(opts *common.GlobalOptions) *cobra.Command {
	var snapshotID string
	var latest bool
	var dryRun bool
	var allowProdSync bool
	var enforceStageRollout bool
	var zoneID, accountID string
	c := &cobra.Command{
		Use:   "dns-restore",
		Short: "Restore router DNS/LB state from a previous sync snapshot",
		Long:  "Restores router DNS/LB state by replaying a previously saved routing snapshot. By default it restores the previous applied snapshot (last-known-good before the latest apply).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router dns-restore", err)
			}
			history, err := app.LoadRouterSyncHistory(ctx.RootDir, opts.Stage)
			if err != nil {
				return common.PrintFailure("router dns-restore", err)
			}
			snapshot, err := app.SelectRouterRestoreSnapshot(history, snapshotID, latest)
			if err != nil {
				return common.PrintFailure("router dns-restore", err)
			}
			routingCfg := app.RouterRoutingConfigFromSnapshot(snapshot)
			if routingCfg == nil {
				return common.PrintFailure("router dns-restore", fmt.Errorf("selected snapshot has no routing payload"))
			}

			policy := app.RouterDNSSyncPolicyForStage(ctx.Config, opts.Stage)
			effectiveAllowProdSync := policy.AllowProdSync
			if cmd.Flags().Changed("allow-prod-dns-sync") {
				effectiveAllowProdSync = allowProdSync
			}
			effectiveEnforceRollout := policy.EnforceStageRollout
			if cmd.Flags().Changed("enforce-dns-sync-stage-rollout") {
				effectiveEnforceRollout = enforceStageRollout
			}
			if err := enforceDNSSyncStageGateWithPolicy(opts.Stage, effectiveAllowProdSync, effectiveEnforceRollout, policy.ApprovalEnvByStage, policy.RequireReason && !dryRun, policy.ReasonEnv); err != nil {
				return common.PrintFailure("router dns-restore", err)
			}

			providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID, policy.ZoneIDEnv, policy.AccountIDEnv)
			if strings.TrimSpace(providerZoneID) == "" {
				providerZoneID = snapshot.ZoneID
			}
			if strings.TrimSpace(providerAccountID) == "" {
				providerAccountID = snapshot.AccountID
			}

			result, err := runRouterDNSSyncWithPolicy(ctx, routingCfg, providerZoneID, providerAccountID, dryRun, policy, cmd.OutOrStdout())
			if err != nil {
				return common.PrintFailure("router dns-restore", err)
			}
			summary := app.RouterSyncSummaryFromResult(result)
			payload := map[string]any{
				"restoredFromSnapshotId": snapshot.ID,
				"dryRun":                 dryRun,
				"summary":                summary,
				"result":                 result,
			}
			historyAnalytics := loadRouterHistoryAnalytics(ctx.RootDir, opts.Stage, 5)
			if historyAnalytics != nil {
				payload["historyAnalytics"] = historyAnalytics
			}
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router dns-restore", payload)
			}
			if dryRun {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"Restore preview from snapshot %s: %d create, %d update, %d unchanged, %d delete-candidate.\n",
					snapshot.ID,
					summary.Create,
					summary.Update,
					summary.Noop,
					summary.DeleteCandidate,
				)
				printRouterResourceBreakdown(cmd.OutOrStdout(), summary)
				if historyAnalytics != nil {
					printRouterHistoryTrend(cmd.OutOrStdout(), *historyAnalytics)
				}
				return nil
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Restore applied from snapshot %s: %d created, %d updated, %d unchanged, %d delete-candidate.\n",
				snapshot.ID,
				summary.Create,
				summary.Update,
				summary.Noop,
				summary.DeleteCandidate,
			)
			printRouterResourceBreakdown(cmd.OutOrStdout(), summary)
			if historyAnalytics != nil {
				printRouterHistoryTrend(cmd.OutOrStdout(), *historyAnalytics)
			}
			return nil
		},
	}
	c.Flags().StringVar(&snapshotID, "snapshot-id", "", "Restore from a specific router sync snapshot ID")
	c.Flags().BoolVar(&latest, "latest", false, "Restore the latest snapshot instead of previous applied snapshot")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Preview restore changes without applying them")
	c.Flags().BoolVar(&allowProdSync, "allow-prod-dns-sync", false, "Required to allow router DNS/LB sync when --stage prod")
	c.Flags().BoolVar(&enforceStageRollout, "enforce-dns-sync-stage-rollout", false, "Enforce staged rollout policy for DNS sync (dev -> staging -> prod)")
	c.Flags().StringVar(&zoneID, "zone-id", "", "Router zone ID (overrides RUNFABRIC_ROUTER_ZONE_ID)")
	c.Flags().StringVar(&accountID, "account-id", "", "Router account ID for LB operations (overrides RUNFABRIC_ROUTER_ACCOUNT_ID)")
	return c
}

func newRouteDNSHistoryCmd(opts *common.GlobalOptions) *cobra.Command {
	var window int
	c := &cobra.Command{
		Use:   "dns-history",
		Short: "Show router DNS sync history analytics and trend summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := app.Bootstrap(opts.ConfigPath, opts.Stage, "")
			if err != nil {
				return common.PrintFailure("router dns-history", err)
			}
			history, err := app.LoadRouterSyncHistory(ctx.RootDir, opts.Stage)
			if err != nil {
				return common.PrintFailure("router dns-history", err)
			}
			analytics := app.AnalyzeRouterSyncHistory(history, window)
			payload := map[string]any{
				"historyAnalytics": analytics,
			}
			if opts.JSONOutput {
				return common.PrintJSONSuccess("router dns-history", payload)
			}
			if len(history) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No router sync history snapshots found for stage %q.\n", opts.Stage)
				return nil
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Router DNS sync history (%s): snapshots=%d applied=%d dry-run=%d drift=%d (%.1f%%)\n",
				opts.Stage,
				analytics.Total.Snapshots,
				analytics.Total.Applied,
				analytics.Total.DryRun,
				analytics.Total.Drift,
				analytics.Total.DriftRate*100,
			)
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Action totals: create=%d update=%d unchanged=%d delete-candidate=%d\n",
				analytics.Total.Create,
				analytics.Total.Update,
				analytics.Total.Noop,
				analytics.Total.DeleteCandidate,
			)
			printRouterHistoryTrend(cmd.OutOrStdout(), analytics)
			printRouterResourceBreakdown(cmd.OutOrStdout(), app.RouterSyncSummary{ByResource: analytics.ByResource})
			if strings.TrimSpace(analytics.LastSnapshotID) != "" {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"Latest snapshot: id=%s operation=%s trigger=%s at=%s\n",
					analytics.LastSnapshotID,
					analytics.LastOperation,
					analytics.LastTrigger,
					analytics.LastSnapshotAt,
				)
			}
			return nil
		},
	}
	c.Flags().IntVar(&window, "window", 5, "Window size used for recent/previous trend comparison")
	return c
}

func enforceDNSSyncStageGate(stage string, allowProdSync, enforceRollout bool) error {
	return enforceDNSSyncStageGateWithPolicy(stage, allowProdSync, enforceRollout, defaultApprovalEnvByStage(), false, "")
}

func enforceDNSSyncStageGateWithPolicy(stage string, allowProdSync, enforceRollout bool, approvalEnvByStage map[string]string, requireReason bool, reasonEnv string) error {
	s := strings.ToLower(stage)
	if s == "" {
		s = "dev"
	}

	if s == "prod" && !allowProdSync {
		return fmt.Errorf("router DNS sync for prod requires --allow-prod-dns-sync")
	}
	if requireReason {
		if strings.TrimSpace(reasonEnv) == "" {
			reasonEnv = "RUNFABRIC_DNS_SYNC_REASON"
		}
		if strings.TrimSpace(os.Getenv(reasonEnv)) == "" {
			return fmt.Errorf("router DNS sync requires %s to describe the change reason", reasonEnv)
		}
	}
	if !enforceRollout {
		return nil
	}

	switch s {
	case "dev":
		return nil
	case "staging":
		approvalEnv := lookupApprovalEnv(approvalEnvByStage, "staging", "RUNFABRIC_DNS_SYNC_DEV_APPROVED")
		if strings.ToLower(os.Getenv(approvalEnv)) != "true" {
			return fmt.Errorf("staging DNS sync requires %s=true when --enforce-dns-sync-stage-rollout is enabled", approvalEnv)
		}
	case "prod":
		approvalEnv := lookupApprovalEnv(approvalEnvByStage, "prod", "RUNFABRIC_DNS_SYNC_STAGING_APPROVED")
		if strings.ToLower(os.Getenv(approvalEnv)) != "true" {
			return fmt.Errorf("prod DNS sync requires %s=true when --enforce-dns-sync-stage-rollout is enabled", approvalEnv)
		}
	default:
		return fmt.Errorf("unsupported stage %q for DNS sync rollout policy; expected dev, staging, or prod", stage)
	}

	return nil
}

func defaultApprovalEnvByStage() map[string]string {
	return map[string]string{
		"staging": "RUNFABRIC_DNS_SYNC_DEV_APPROVED",
		"prod":    "RUNFABRIC_DNS_SYNC_STAGING_APPROVED",
	}
}

func lookupApprovalEnv(approvalEnvByStage map[string]string, stage, fallback string) string {
	if approvalEnvByStage == nil {
		return fallback
	}
	v := strings.TrimSpace(approvalEnvByStage[strings.ToLower(strings.TrimSpace(stage))])
	if v == "" {
		return fallback
	}
	return v
}

func resolveDNSProviderIDs(zoneFlag, accountFlag, zoneEnv, accountEnv string) (string, string) {
	if strings.TrimSpace(zoneEnv) == "" {
		zoneEnv = "RUNFABRIC_ROUTER_ZONE_ID"
	}
	if strings.TrimSpace(accountEnv) == "" {
		accountEnv = "RUNFABRIC_ROUTER_ACCOUNT_ID"
	}
	zoneID := zoneFlag
	if zoneID == "" {
		zoneID = os.Getenv(zoneEnv)
	}
	accountID := accountFlag
	if accountID == "" {
		accountID = os.Getenv(accountEnv)
	}
	return zoneID, accountID
}

func runRouterDNSSyncWithPolicy(
	ctx *app.AppContext,
	routingCfg *app.RouterRoutingConfig,
	zoneID, accountID string,
	dryRun bool,
	policy app.RouterDNSSyncPolicy,
	out io.Writer,
) (*routercontracts.SyncResult, error) {
	operationID := newRouterOperationID()
	events := []state.RouterSyncEvent{
		{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Phase:     "start",
			Message:   fmt.Sprintf("router sync started (dryRun=%t)", dryRun),
		},
	}
	if err := primeRouterAPIToken(ctx, policy); err != nil {
		return nil, fmt.Errorf("router sync %s: %w", operationID, err)
	}
	if err := enforceRouterCredentialPolicy(policy.CredentialPolicy, time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("router sync %s: %w", operationID, err)
	}
	var beforeActions []routercontracts.SyncAction
	if !dryRun {
		preview, err := previewRouterDNSSync(ctx, routingCfg, zoneID, accountID)
		if err != nil {
			return nil, fmt.Errorf("router sync %s preflight failed: %w", operationID, err)
		}
		beforeActions = preview.Actions
		events = append(events, state.RouterSyncEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Phase:     "preflight",
			Message:   "captured before-state action snapshot",
			Summary:   toStateRouterActionSummary(app.RouterSyncSummaryFromResult(preview)),
		})
		if policy.MutationPolicy.Enabled {
			if err := enforceRouterMutationPolicy(policy.MutationPolicy, preview); err != nil {
				return nil, fmt.Errorf("router sync %s: %w", operationID, err)
			}
		}
	}
	result, err := app.RouterDNSSyncWithOptions(
		ctx,
		routingCfg,
		zoneID,
		accountID,
		dryRun,
		out,
		app.RouterDNSSyncOptions{
			OperationID:   operationID,
			Trigger:       "cli-router",
			BeforeActions: beforeActions,
			Events:        events,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("router sync %s failed: %w", operationID, err)
	}
	if out != nil {
		fmt.Fprintf(out, "router operation id: %s\n", operationID)
	}
	return result, nil
}

func previewRouterDNSSync(
	ctx *app.AppContext,
	routingCfg *app.RouterRoutingConfig,
	zoneID, accountID string,
) (*routercontracts.SyncResult, error) {
	if ctx == nil || ctx.Extensions == nil {
		return nil, fmt.Errorf("app context extensions are not initialized")
	}
	pluginID := app.SelectedRouterPlugin(ctx.Config)
	return ctx.Extensions.SyncRouter(context.Background(), pluginID, app.RouterSyncRequest{
		Routing:   routingCfg,
		ZoneID:    zoneID,
		AccountID: accountID,
		DryRun:    true,
		Out:       io.Discard,
	})
}

func primeRouterAPIToken(ctx *app.AppContext, policy app.RouterDNSSyncPolicy) error {
	if strings.TrimSpace(os.Getenv("RUNFABRIC_ROUTER_API_TOKEN")) != "" {
		return nil
	}
	apiTokenEnv := strings.TrimSpace(policy.APITokenEnv)
	if apiTokenEnv == "" {
		apiTokenEnv = "RUNFABRIC_ROUTER_API_TOKEN"
	}
	if v := strings.TrimSpace(os.Getenv(apiTokenEnv)); v != "" {
		return os.Setenv("RUNFABRIC_ROUTER_API_TOKEN", v)
	}
	if strings.TrimSpace(policy.APITokenSecretRef) != "" {
		var token string
		var err error
		if ctx != nil {
			token, err = app.ResolveRouterAPITokenSecretRef(ctx.Config, policy.APITokenSecretRef)
		} else {
			token, err = app.ResolveRouterAPITokenSecretRef(nil, policy.APITokenSecretRef)
		}
		if err != nil {
			return fmt.Errorf("resolve router API token secret ref: %w", err)
		}
		return os.Setenv("RUNFABRIC_ROUTER_API_TOKEN", token)
	}
	apiTokenFileEnv := strings.TrimSpace(policy.APITokenFileEnv)
	if apiTokenFileEnv == "" {
		apiTokenFileEnv = "RUNFABRIC_ROUTER_API_TOKEN_FILE"
	}
	path := strings.TrimSpace(os.Getenv(apiTokenFileEnv))
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read router API token file from %s: %w", apiTokenFileEnv, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return fmt.Errorf("router API token file from %s is empty", apiTokenFileEnv)
	}
	return os.Setenv("RUNFABRIC_ROUTER_API_TOKEN", token)
}

func enforceRouterCredentialPolicy(policy app.RouterCredentialPolicy, now time.Time) error {
	if !policy.Enabled {
		return nil
	}
	attestationEnv := strings.TrimSpace(policy.AttestationEnv)
	if attestationEnv == "" {
		attestationEnv = "RUNFABRIC_ROUTER_TOKEN_ATTESTED"
	}
	if policy.RequireAttestation && !strings.EqualFold(strings.TrimSpace(os.Getenv(attestationEnv)), "true") {
		return fmt.Errorf("router credential policy requires %s=true", attestationEnv)
	}

	issuedAtEnv := strings.TrimSpace(policy.IssuedAtEnv)
	if issuedAtEnv == "" {
		issuedAtEnv = "RUNFABRIC_ROUTER_TOKEN_ISSUED_AT"
	}
	expiresAtEnv := strings.TrimSpace(policy.ExpiresAtEnv)
	if expiresAtEnv == "" {
		expiresAtEnv = "RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT"
	}
	issuedRaw := strings.TrimSpace(os.Getenv(issuedAtEnv))
	expiresRaw := strings.TrimSpace(os.Getenv(expiresAtEnv))

	var issuedAt time.Time
	var expiresAt time.Time
	var err error
	if issuedRaw != "" {
		issuedAt, err = time.Parse(time.RFC3339, issuedRaw)
		if err != nil {
			return fmt.Errorf("router credential policy expects %s in RFC3339 format", issuedAtEnv)
		}
	}
	if expiresRaw != "" {
		expiresAt, err = time.Parse(time.RFC3339, expiresRaw)
		if err != nil {
			return fmt.Errorf("router credential policy expects %s in RFC3339 format", expiresAtEnv)
		}
	}

	if policy.MaxTTLSeconds > 0 {
		if issuedRaw == "" || expiresRaw == "" {
			return fmt.Errorf(
				"router credential policy requires %s and %s when maxTTLSeconds=%d",
				issuedAtEnv,
				expiresAtEnv,
				policy.MaxTTLSeconds,
			)
		}
		ttl := expiresAt.Sub(issuedAt)
		if ttl <= 0 {
			return fmt.Errorf("router credential policy requires %s to be after %s", expiresAtEnv, issuedAtEnv)
		}
		maxTTL := time.Duration(policy.MaxTTLSeconds) * time.Second
		if ttl > maxTTL {
			return fmt.Errorf(
				"router credential policy rejected token TTL=%s; max allowed is %ds",
				ttl.Truncate(time.Second),
				policy.MaxTTLSeconds,
			)
		}
	}

	if policy.MinRemainingSeconds > 0 {
		if expiresRaw == "" {
			return fmt.Errorf(
				"router credential policy requires %s when minRemainingSeconds=%d",
				expiresAtEnv,
				policy.MinRemainingSeconds,
			)
		}
		remaining := expiresAt.Sub(now)
		minRemaining := time.Duration(policy.MinRemainingSeconds) * time.Second
		if remaining < minRemaining {
			return fmt.Errorf(
				"router credential policy requires at least %ds remaining lifetime; current=%s",
				policy.MinRemainingSeconds,
				remaining.Truncate(time.Second),
			)
		}
	}
	if !expiresAt.IsZero() && now.After(expiresAt) {
		return fmt.Errorf("router credential policy rejected expired token from %s", expiresAtEnv)
	}
	return nil
}

func enforceRouterMutationPolicy(policy app.RouterMutationPolicy, preview *routercontracts.SyncResult) error {
	if !policy.Enabled || preview == nil {
		return nil
	}
	approvalEnv := strings.TrimSpace(policy.ApprovalEnv)
	if approvalEnv == "" {
		approvalEnv = "RUNFABRIC_DNS_SYNC_RISK_APPROVED"
	}
	approved := strings.EqualFold(strings.TrimSpace(os.Getenv(approvalEnv)), "true")

	riskyResources := map[string]struct{}{}
	for _, resource := range policy.RiskyResources {
		r := strings.ToLower(strings.TrimSpace(resource))
		if r != "" {
			riskyResources[r] = struct{}{}
		}
	}

	mutations := 0
	riskyMutations := 0
	riskyNames := map[string]struct{}{}
	for _, action := range preview.Actions {
		op := strings.ToLower(strings.TrimSpace(action.Action))
		switch op {
		case "create", "update":
			mutations++
			resource := strings.ToLower(strings.TrimSpace(action.Resource))
			if _, ok := riskyResources[resource]; ok {
				riskyMutations++
				riskyNames[resource] = struct{}{}
			}
		}
	}

	if policy.MaxMutationsWithoutApproval > 0 && mutations > policy.MaxMutationsWithoutApproval && !approved {
		return fmt.Errorf(
			"router mutation policy requires %s=true: planned mutations=%d exceed maxMutationsWithoutApproval=%d",
			approvalEnv,
			mutations,
			policy.MaxMutationsWithoutApproval,
		)
	}
	if riskyMutations > 0 && !approved {
		resources := make([]string, 0, len(riskyNames))
		for name := range riskyNames {
			resources = append(resources, name)
		}
		return fmt.Errorf(
			"router mutation policy requires %s=true: planned risky mutations=%d on resources=%s",
			approvalEnv,
			riskyMutations,
			strings.Join(resources, ","),
		)
	}
	return nil
}

func endpointWeightMap(routingCfg *app.RouterRoutingConfig) map[string]int {
	out := map[string]int{}
	if routingCfg == nil {
		return out
	}
	for _, ep := range routingCfg.Endpoints {
		name := strings.TrimSpace(ep.Name)
		if name == "" {
			continue
		}
		out[name] = ep.Weight
	}
	return out
}

func printDistribution(out io.Writer, distribution map[string]int) {
	if out == nil || len(distribution) == 0 {
		return
	}
	keys := make([]string, 0, len(distribution))
	for k := range distribution {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(out, "  %s: %d\n", k, distribution[k])
	}
}

func printRouterResourceBreakdown(out io.Writer, summary app.RouterSyncSummary) {
	if out == nil || len(summary.ByResource) == 0 {
		return
	}
	fmt.Fprintf(out, "Resource-level summary:\n")
	keys := make([]string, 0, len(summary.ByResource))
	for resource := range summary.ByResource {
		keys = append(keys, resource)
	}
	sort.Strings(keys)
	for _, resource := range keys {
		item := summary.ByResource[resource]
		fmt.Fprintf(
			out,
			"  %s: create=%d update=%d unchanged=%d delete-candidate=%d\n",
			resource,
			item.Create,
			item.Update,
			item.Noop,
			item.DeleteCandidate,
		)
	}
}

func loadRouterHistoryAnalytics(root, stage string, window int) *app.RouterSyncHistoryAnalytics {
	history, err := app.LoadRouterSyncHistory(root, stage)
	if err != nil {
		return nil
	}
	analytics := app.AnalyzeRouterSyncHistory(history, window)
	return &analytics
}

func printRouterHistoryTrend(out io.Writer, analytics app.RouterSyncHistoryAnalytics) {
	if out == nil || analytics.Total.Snapshots == 0 {
		return
	}
	fmt.Fprintf(
		out,
		"History trend (window=%d): %s | recent mutation-rate=%.2f, previous mutation-rate=%.2f\n",
		analytics.Window,
		analytics.Trend,
		analytics.Recent.MutationRate,
		analytics.Previous.MutationRate,
	)
}

func newRouterOperationID() string {
	return fmt.Sprintf("router-sync-%d", time.Now().UTC().UnixNano())
}

func toStateRouterActionSummary(summary app.RouterSyncSummary) state.RouterSyncActionSummary {
	return state.RouterSyncActionSummary{
		Create:          summary.Create,
		Update:          summary.Update,
		Noop:            summary.Noop,
		DeleteCandidate: summary.DeleteCandidate,
	}
}
