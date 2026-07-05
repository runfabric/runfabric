package app

import (
	"fmt"
	"io"

	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// routerRoutingFromFabricState bootstraps the app context and rebuilds the
// multi-cloud routing config from the fabric state recorded by RunFabricDeploy.
// Shared by the daemon-facing router operations (sync/simulate/verify/shift).
func routerRoutingFromFabricState(configPath, stage string) (*AppContext, *RouterRoutingConfig, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, nil, err
	}
	fabricState, err := state.LoadRunFabricState(ctx.RootDir, stage)
	if err != nil {
		return nil, nil, fmt.Errorf("load fabric state: %w", err)
	}
	if fabricState == nil || len(fabricState.Endpoints) == 0 {
		return nil, nil, fmt.Errorf("no fabric endpoints recorded for stage %q — run a fabric deploy (fabric.targets + providerOverrides) first", stage)
	}
	routing := GenerateRouterRoutingConfig(fabricState, ctx.Config, stage)
	if routing == nil || len(routing.Endpoints) == 0 {
		return nil, nil, fmt.Errorf("fabric routing config is empty — check fabric configuration in %s", configPath)
	}
	return ctx, routing, nil
}

// RouterSimulateFromFabricState replays synthetic traffic against the recorded
// fabric routing config locally — no provider API calls.
func RouterSimulateFromFabricState(configPath, stage string, requests int, down []string) (*RouterSimulationResult, error) {
	_, routing, err := routerRoutingFromFabricState(configPath, stage)
	if err != nil {
		return nil, err
	}
	result := SimulateRouterRouting(routing, requests, down)
	return &result, nil
}

// RouterVerifyFailoverFromFabricState runs the one-endpoint-down and
// all-endpoints-down chaos scenarios against the recorded fabric routing config.
func RouterVerifyFailoverFromFabricState(configPath, stage string, requests int) (*RouterChaosVerification, error) {
	_, routing, err := routerRoutingFromFabricState(configPath, stage)
	if err != nil {
		return nil, err
	}
	report := VerifyRouterFailover(routing, requests)
	return &report, nil
}

// RouterShiftFromFabricState applies a progressive canary weight to one fabric
// endpoint and syncs the shifted routing config through the router plugin.
// dryRun true (or the stage policy's dryRun) previews without mutating DNS.
func RouterShiftFromFabricState(configPath, stage, provider string, percent int, dryRun bool, out io.Writer) (map[string]any, error) {
	ctx, routing, err := routerRoutingFromFabricState(configPath, stage)
	if err != nil {
		return nil, err
	}
	if !ApplyCanaryWeights(routing, provider, percent) {
		return nil, fmt.Errorf("canary provider %q not found in router endpoints", provider)
	}
	policy := RouterDNSSyncPolicyForStage(ctx.Config, stage)
	zoneID, accountID := RouterProviderIDs(policy)
	effectiveDryRun := dryRun || policy.DryRun
	if out == nil {
		out = io.Discard
	}
	result, err := RouterDNSSyncWithOptions(ctx, routing, zoneID, accountID, effectiveDryRun, out, RouterDNSSyncOptions{
		Trigger: "canary-shift",
	})
	if err != nil {
		return nil, err
	}
	weights := map[string]int{}
	for _, ep := range routing.Endpoints {
		weights[ep.Name] = ep.Weight
	}
	return map[string]any{
		"provider": provider,
		"percent":  percent,
		"dryRun":   effectiveDryRun,
		"weights":  weights,
		"result":   result,
	}, nil
}

// RouterHistoryFromFabricState returns the stage's router sync snapshots plus
// drift/apply analytics over the most recent window.
func RouterHistoryFromFabricState(configPath, stage string, recentWindow int) (map[string]any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	history, err := LoadRouterSyncHistory(ctx.RootDir, stage)
	if err != nil {
		return nil, err
	}
	if recentWindow <= 0 {
		recentWindow = 5
	}
	return map[string]any{
		"history":   history,
		"analytics": AnalyzeRouterSyncHistory(history, recentWindow),
	}, nil
}

// RouterRestoreFromSnapshot replays a previously saved routing snapshot through
// the router plugin (last-known-good rollback). snapshotID selects a specific
// snapshot; latest selects the newest applied one; both empty/false selects the
// previous applied snapshot. Zone/account default to the snapshot's own ids.
func RouterRestoreFromSnapshot(configPath, stage, snapshotID string, latest, dryRun bool, out io.Writer) (map[string]any, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	history, err := LoadRouterSyncHistory(ctx.RootDir, stage)
	if err != nil {
		return nil, err
	}
	snapshot, err := SelectRouterRestoreSnapshot(history, snapshotID, latest)
	if err != nil {
		return nil, err
	}
	routing := RouterRoutingConfigFromSnapshot(snapshot)
	if routing == nil {
		return nil, fmt.Errorf("selected snapshot has no routing payload")
	}
	policy := RouterDNSSyncPolicyForStage(ctx.Config, stage)
	zoneID, accountID := RouterProviderIDs(policy)
	if zoneID == "" {
		zoneID = snapshot.ZoneID
	}
	if accountID == "" {
		accountID = snapshot.AccountID
	}
	effectiveDryRun := dryRun || policy.DryRun
	if out == nil {
		out = io.Discard
	}
	result, err := RouterDNSSyncWithOptions(ctx, routing, zoneID, accountID, effectiveDryRun, out, RouterDNSSyncOptions{
		Trigger: "restore",
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"restoredFromSnapshotId": snapshot.ID,
		"dryRun":                 effectiveDryRun,
		"summary":                RouterSyncSummaryFromResult(result),
		"result":                 result,
	}, nil
}
