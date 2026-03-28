package app

import (
	"fmt"
	"math"
	"strings"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	statecore "github.com/runfabric/runfabric/platform/core/state/core"
)

type RouterSyncSummary struct {
	Create          int                              `json:"create"`
	Update          int                              `json:"update"`
	Noop            int                              `json:"noop"`
	DeleteCandidate int                              `json:"deleteCandidate,omitempty"`
	ByResource      map[string]RouterResourceSummary `json:"byResource,omitempty"`
	DriftDetected   bool                             `json:"driftDetected"`
}

type RouterResourceSummary struct {
	Create          int `json:"create"`
	Update          int `json:"update"`
	Noop            int `json:"noop"`
	DeleteCandidate int `json:"deleteCandidate,omitempty"`
}

type RouterSyncTrendSummary struct {
	Snapshots       int     `json:"snapshots"`
	Applied         int     `json:"applied"`
	DryRun          int     `json:"dryRun"`
	Drift           int     `json:"drift"`
	Create          int     `json:"create"`
	Update          int     `json:"update"`
	Noop            int     `json:"noop"`
	DeleteCandidate int     `json:"deleteCandidate,omitempty"`
	DriftRate       float64 `json:"driftRate"`
	MutationRate    float64 `json:"mutationRate"`
}

type RouterSyncHistoryAnalytics struct {
	Window         int                              `json:"window"`
	Trend          string                           `json:"trend"`
	LastSnapshotID string                           `json:"lastSnapshotId,omitempty"`
	LastSnapshotAt string                           `json:"lastSnapshotAt,omitempty"`
	LastOperation  string                           `json:"lastOperation,omitempty"`
	LastTrigger    string                           `json:"lastTrigger,omitempty"`
	Total          RouterSyncTrendSummary           `json:"total"`
	Recent         RouterSyncTrendSummary           `json:"recent"`
	Previous       RouterSyncTrendSummary           `json:"previous"`
	ByResource     map[string]RouterResourceSummary `json:"byResource,omitempty"`
}

func RouterSyncSummaryFromResult(result *routercontracts.SyncResult) RouterSyncSummary {
	if result == nil {
		return RouterSyncSummary{}
	}
	summary := RouterSyncSummary{
		ByResource: map[string]RouterResourceSummary{},
	}
	for _, action := range result.Actions {
		accumulateRouterAction(&summary, action.Resource, action.Action)
	}
	summary.DriftDetected = summary.Create > 0 || summary.Update > 0 || summary.DeleteCandidate > 0
	return summary
}

func RouterSyncSummaryFromSnapshot(snapshot statecore.RouterSyncSnapshot) RouterSyncSummary {
	summary := RouterSyncSummary{
		ByResource: map[string]RouterResourceSummary{},
	}
	for _, action := range snapshot.Actions {
		accumulateRouterAction(&summary, action.Resource, action.Action)
	}
	summary.DriftDetected = summary.Create > 0 || summary.Update > 0 || summary.DeleteCandidate > 0
	return summary
}

func AnalyzeRouterSyncHistory(history []statecore.RouterSyncSnapshot, recentWindow int) RouterSyncHistoryAnalytics {
	if recentWindow <= 0 {
		recentWindow = 5
	}
	analytics := RouterSyncHistoryAnalytics{
		Window:     recentWindow,
		Trend:      "insufficient-data",
		ByResource: map[string]RouterResourceSummary{},
	}
	total, byResource := summarizeRouterSnapshotWindow(history)
	analytics.Total = total
	analytics.ByResource = byResource

	if len(history) == 0 {
		return analytics
	}
	last := history[len(history)-1]
	analytics.LastSnapshotID = strings.TrimSpace(last.ID)
	analytics.LastSnapshotAt = strings.TrimSpace(last.CreatedAt)
	analytics.LastOperation = strings.TrimSpace(last.Operation)
	analytics.LastTrigger = strings.TrimSpace(last.Trigger)

	recentStart := len(history) - recentWindow
	if recentStart < 0 {
		recentStart = 0
	}
	analytics.Recent, _ = summarizeRouterSnapshotWindow(history[recentStart:])
	if recentStart > 0 {
		prevStart := recentStart - recentWindow
		if prevStart < 0 {
			prevStart = 0
		}
		analytics.Previous, _ = summarizeRouterSnapshotWindow(history[prevStart:recentStart])
	}
	analytics.Trend = classifyRouterHistoryTrend(analytics.Previous, analytics.Recent)
	return analytics
}

func accumulateRouterAction(summary *RouterSyncSummary, resource, action string) {
	if summary == nil {
		return
	}
	if summary.ByResource == nil {
		summary.ByResource = map[string]RouterResourceSummary{}
	}
	resource = strings.ToLower(strings.TrimSpace(resource))
	if resource == "" {
		resource = "unknown"
	}
	resourceSummary := summary.ByResource[resource]
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "create":
		summary.Create++
		resourceSummary.Create++
	case "update":
		summary.Update++
		resourceSummary.Update++
	case "no-op":
		summary.Noop++
		resourceSummary.Noop++
	case "delete-candidate":
		summary.DeleteCandidate++
		resourceSummary.DeleteCandidate++
	}
	summary.ByResource[resource] = resourceSummary
}

func summarizeRouterSnapshotWindow(history []statecore.RouterSyncSnapshot) (RouterSyncTrendSummary, map[string]RouterResourceSummary) {
	out := RouterSyncTrendSummary{}
	byResource := map[string]RouterResourceSummary{}
	for _, snapshot := range history {
		out.Snapshots++
		if snapshot.DryRun {
			out.DryRun++
		} else {
			out.Applied++
		}
		summary := RouterSyncSummaryFromSnapshot(snapshot)
		out.Create += summary.Create
		out.Update += summary.Update
		out.Noop += summary.Noop
		out.DeleteCandidate += summary.DeleteCandidate
		if summary.DriftDetected {
			out.Drift++
		}
		for resource, item := range summary.ByResource {
			existing := byResource[resource]
			existing.Create += item.Create
			existing.Update += item.Update
			existing.Noop += item.Noop
			existing.DeleteCandidate += item.DeleteCandidate
			byResource[resource] = existing
		}
	}
	if out.Snapshots > 0 {
		out.DriftRate = float64(out.Drift) / float64(out.Snapshots)
		mutations := out.Create + out.Update + out.DeleteCandidate
		out.MutationRate = float64(mutations) / float64(out.Snapshots)
	}
	return out, byResource
}

func classifyRouterHistoryTrend(previous, recent RouterSyncTrendSummary) string {
	if previous.Snapshots == 0 || recent.Snapshots == 0 {
		return "insufficient-data"
	}
	delta := recent.MutationRate - previous.MutationRate
	if math.Abs(delta) <= 0.15 {
		return "stable"
	}
	if delta < 0 {
		return "improving"
	}
	return "degrading"
}

func LoadRouterSyncHistory(root, stage string) ([]statecore.RouterSyncSnapshot, error) {
	return statecore.LoadRouterSyncHistory(root, stage)
}

func RouterRoutingConfigFromSnapshot(snapshot *statecore.RouterSyncSnapshot) *RouterRoutingConfig {
	if snapshot == nil {
		return nil
	}
	routing := &RouterRoutingConfig{
		Contract:   snapshot.Routing.Contract,
		Service:    snapshot.Routing.Service,
		Stage:      snapshot.Routing.Stage,
		Hostname:   snapshot.Routing.Hostname,
		Strategy:   snapshot.Routing.Strategy,
		HealthPath: snapshot.Routing.HealthPath,
		TTL:        snapshot.Routing.TTL,
		Endpoints:  make([]RouterRoutingEndpoint, len(snapshot.Routing.Endpoints)),
	}
	for i, ep := range snapshot.Routing.Endpoints {
		routing.Endpoints[i] = RouterRoutingEndpoint{
			Name:    ep.Name,
			URL:     ep.URL,
			Weight:  ep.Weight,
			Healthy: ep.Healthy,
		}
	}
	return routing
}

func SelectRouterRestoreSnapshot(history []statecore.RouterSyncSnapshot, snapshotID string, latest bool) (*statecore.RouterSyncSnapshot, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("no router sync snapshots found")
	}
	if id := strings.TrimSpace(snapshotID); id != "" {
		for i := len(history) - 1; i >= 0; i-- {
			if strings.TrimSpace(history[i].ID) == id {
				s := history[i]
				return &s, nil
			}
		}
		return nil, fmt.Errorf("router sync snapshot %q not found", id)
	}
	if latest {
		s := history[len(history)-1]
		return &s, nil
	}
	// Default restore target is the previous non-dry-run snapshot
	// (last-known-good before the most recent apply).
	nonDry := make([]statecore.RouterSyncSnapshot, 0, len(history))
	for _, s := range history {
		if !s.DryRun {
			nonDry = append(nonDry, s)
		}
	}
	if len(nonDry) == 0 {
		return nil, fmt.Errorf("no applied router sync snapshots found")
	}
	if len(nonDry) == 1 {
		s := nonDry[0]
		return &s, nil
	}
	s := nonDry[len(nonDry)-2]
	return &s, nil
}
