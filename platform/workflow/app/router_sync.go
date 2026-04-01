package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	"github.com/runfabric/runfabric/platform/core/model/config"
	statecore "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

type RouterDNSSyncOptions struct {
	OperationID   string
	Trigger       string
	BeforeActions []routercontracts.SyncAction
	Events        []statecore.RouterSyncEvent
}

// SelectedRouterPlugin returns extensions.routerPlugin, defaulting to cloudflare.
func SelectedRouterPlugin(cfg *config.Config) string {
	id := strings.TrimSpace(config.ExtensionString(cfg, "routerPlugin"))
	if id == "" {
		return "cloudflare"
	}
	if catalog, err := resolution.DiscoverPluginCatalog(external.DiscoverOptions{}); err == nil && catalog != nil {
		if normalizedID, nerr := normalizePluginIDByKind(manifests.KindRouter, id, catalog.Registry); nerr == nil && strings.TrimSpace(normalizedID) != "" {
			return strings.ToLower(strings.TrimSpace(normalizedID))
		}
	}
	return strings.ToLower(id)
}

// RouterDNSSync dispatches router DNS/LB sync through the configured router plugin.
func RouterDNSSync(ctx *AppContext, routing *RouterRoutingConfig, zoneID, accountID string, dryRun bool, out io.Writer) (*routercontracts.SyncResult, error) {
	return RouterDNSSyncWithOptions(ctx, routing, zoneID, accountID, dryRun, out, RouterDNSSyncOptions{})
}

// RouterDNSSyncWithOptions dispatches router DNS/LB sync and persists snapshots with optional operation metadata.
func RouterDNSSyncWithOptions(
	ctx *AppContext,
	routing *RouterRoutingConfig,
	zoneID, accountID string,
	dryRun bool,
	out io.Writer,
	options RouterDNSSyncOptions,
) (*routercontracts.SyncResult, error) {
	if ctx == nil || ctx.Extensions == nil {
		return nil, fmt.Errorf("app context extensions are not initialized")
	}
	pluginID := SelectedRouterPlugin(ctx.Config)
	result, err := ctx.Extensions.SyncRouter(context.Background(), pluginID, RouterSyncRequest{
		Routing:   routing,
		ZoneID:    zoneID,
		AccountID: accountID,
		DryRun:    dryRun,
		Out:       out,
	})
	if err != nil {
		return nil, err
	}
	if routing == nil {
		return result, nil
	}
	stage := strings.TrimSpace(routing.Stage)
	if stage == "" {
		stage = strings.TrimSpace(ctx.Stage)
	}
	snapshot := statecore.RouterSyncSnapshot{
		Service:   routing.Service,
		Stage:     stage,
		PluginID:  pluginID,
		Operation: strings.TrimSpace(options.OperationID),
		Trigger:   strings.TrimSpace(options.Trigger),
		ZoneID:    zoneID,
		AccountID: accountID,
		DryRun:    dryRun,
		Routing:   toStateRouterSyncRouting(routing),
	}
	if len(options.BeforeActions) > 0 {
		snapshot.Before = toStateRouterSyncActions(options.BeforeActions)
		snapshot.BeforeSum = summarizeStateRouterSyncActions(snapshot.Before)
	}
	if len(options.Events) > 0 {
		snapshot.Events = append(snapshot.Events, options.Events...)
	}
	if result != nil {
		snapshot.Actions = toStateRouterSyncActions(result.Actions)
		snapshot.After = snapshot.Actions
		snapshot.Summary = summarizeStateRouterSyncActions(snapshot.Actions)
		snapshot.AfterSum = snapshot.Summary
		snapshot.Events = append(snapshot.Events, statecore.RouterSyncEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Phase:     "complete",
			Message:   "router sync completed",
			Summary:   snapshot.Summary,
		})
	}
	if err := statecore.AppendRouterSyncSnapshot(ctx.RootDir, stage, snapshot, 30); err != nil {
		return nil, err
	}
	return result, nil
}

func toStateRouterSyncRouting(routing *RouterRoutingConfig) statecore.RouterSyncRouting {
	out := statecore.RouterSyncRouting{
		Contract:   routing.Contract,
		Service:    routing.Service,
		Stage:      routing.Stage,
		Hostname:   routing.Hostname,
		Strategy:   routing.Strategy,
		HealthPath: routing.HealthPath,
		TTL:        routing.TTL,
		Endpoints:  make([]statecore.RouterSyncEndpoint, len(routing.Endpoints)),
	}
	for i, ep := range routing.Endpoints {
		out.Endpoints[i] = statecore.RouterSyncEndpoint{
			Name:    ep.Name,
			URL:     ep.URL,
			Weight:  ep.Weight,
			Healthy: ep.Healthy,
		}
	}
	return out
}

func toStateRouterSyncActions(actions []routercontracts.SyncAction) []statecore.RouterSyncAction {
	out := make([]statecore.RouterSyncAction, len(actions))
	for i, a := range actions {
		out[i] = statecore.RouterSyncAction{
			Resource: a.Resource,
			Action:   a.Action,
			Name:     a.Name,
			Detail:   a.Detail,
		}
	}
	return out
}

func summarizeStateRouterSyncActions(actions []statecore.RouterSyncAction) statecore.RouterSyncActionSummary {
	summary := statecore.RouterSyncActionSummary{}
	for _, action := range actions {
		switch strings.ToLower(strings.TrimSpace(action.Action)) {
		case "create":
			summary.Create++
		case "update":
			summary.Update++
		case "no-op":
			summary.Noop++
		case "delete-candidate":
			summary.DeleteCandidate++
		}
	}
	return summary
}
