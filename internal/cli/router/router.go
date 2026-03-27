package router

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/runfabric/runfabric/internal/cli/common"

	"github.com/runfabric/runfabric/internal/app"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/spf13/cobra"
)

func newRouteCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "router",
		Aliases: []string{"fabric"},
		Short:   "Runtime router: active-active deploy, health check, endpoints",
		Long:    "When runfabric.yml has fabric.targets (provider keys from providerOverrides), deploy to multiple targets and run health checks. Use router deploy for active-active; router status to check endpoint health; router endpoints to list URLs (e.g. for failover/latency routing).",
	}
	cmd.AddCommand(newRouteDeployCmd(opts), newRouteStatusCmd(opts), newRouteEndpointsCmd(opts), newRouteRoutingCmd(opts), newRouteDNSSyncCmd(opts))
	return cmd
}

func newRouteDeployCmd(opts *GlobalOptions) *cobra.Command {
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

			if syncDNS {
				if err := enforceDNSSyncStageGate(opts.Stage, allowProdSync, enforceStageRollout); err != nil {
					return common.PrintFailure("router deploy", err)
				}
				if !opts.JSONOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Running post-deploy router DNS sync via plugin %q...\n", app.SelectedRouterPlugin(ctx.Config))
				}
				routingCfg := app.GenerateRouterRoutingConfig(fabricState, ctx.Config, opts.Stage)
				providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID)
				result, err := app.RouterDNSSync(ctx, routingCfg, providerZoneID, providerAccountID, syncDryRun, cmd.OutOrStdout())
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

func newRouteStatusCmd(opts *GlobalOptions) *cobra.Command {
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

func newRouteEndpointsCmd(opts *GlobalOptions) *cobra.Command {
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

func newRouteRoutingCmd(opts *GlobalOptions) *cobra.Command {
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

func newRouteDNSSyncCmd(opts *GlobalOptions) *cobra.Command {
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
			if err := enforceDNSSyncStageGate(opts.Stage, allowProdSync, enforceStageRollout); err != nil {
				return common.PrintFailure("router dns-sync", err)
			}

			providerZoneID, providerAccountID := resolveDNSProviderIDs(zoneID, accountID)

			result, err := app.RouterDNSSync(ctx, routingCfg, providerZoneID, providerAccountID, dryRun, cmd.OutOrStdout())
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

func enforceDNSSyncStageGate(stage string, allowProdSync, enforceRollout bool) error {
	s := strings.ToLower(stage)

	if s == "prod" && !allowProdSync {
		return fmt.Errorf("router DNS sync for prod requires --allow-prod-dns-sync")
	}
	if !enforceRollout {
		return nil
	}

	switch s {
	case "dev", "":
		return nil
	case "staging":
		if strings.ToLower(os.Getenv("RUNFABRIC_DNS_SYNC_DEV_APPROVED")) != "true" {
			return fmt.Errorf("staging DNS sync requires RUNFABRIC_DNS_SYNC_DEV_APPROVED=true when --enforce-dns-sync-stage-rollout is enabled")
		}
	case "prod":
		if strings.ToLower(os.Getenv("RUNFABRIC_DNS_SYNC_STAGING_APPROVED")) != "true" {
			return fmt.Errorf("prod DNS sync requires RUNFABRIC_DNS_SYNC_STAGING_APPROVED=true when --enforce-dns-sync-stage-rollout is enabled")
		}
	default:
		return fmt.Errorf("unsupported stage %q for DNS sync rollout policy; expected dev, staging, or prod", stage)
	}

	return nil
}

func resolveDNSProviderIDs(zoneFlag, accountFlag string) (string, string) {
	zoneID := zoneFlag
	if zoneID == "" {
		zoneID = os.Getenv("RUNFABRIC_ROUTER_ZONE_ID")
	}
	accountID := accountFlag
	if accountID == "" {
		accountID = os.Getenv("RUNFABRIC_ROUTER_ACCOUNT_ID")
	}
	return zoneID, accountID
}
