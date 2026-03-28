package app

import (
	"testing"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	statecore "github.com/runfabric/runfabric/platform/core/state/core"
)

func TestSelectRouterRestoreSnapshot_DefaultPreviousApplied(t *testing.T) {
	history := []statecore.RouterSyncSnapshot{
		{ID: "1", DryRun: false},
		{ID: "2", DryRun: true},
		{ID: "3", DryRun: false},
		{ID: "4", DryRun: false},
	}
	got, err := SelectRouterRestoreSnapshot(history, "", false)
	if err != nil {
		t.Fatalf("select restore snapshot: %v", err)
	}
	if got.ID != "3" {
		t.Fatalf("expected previous applied snapshot id=3, got %q", got.ID)
	}
}

func TestSelectRouterRestoreSnapshot_Latest(t *testing.T) {
	history := []statecore.RouterSyncSnapshot{{ID: "1", DryRun: false}, {ID: "2", DryRun: false}}
	got, err := SelectRouterRestoreSnapshot(history, "", true)
	if err != nil {
		t.Fatalf("select latest snapshot: %v", err)
	}
	if got.ID != "2" {
		t.Fatalf("expected latest snapshot id=2, got %q", got.ID)
	}
}

func TestSelectRouterRestoreSnapshot_ByID(t *testing.T) {
	history := []statecore.RouterSyncSnapshot{{ID: "snap-a", DryRun: false}, {ID: "snap-b", DryRun: false}}
	got, err := SelectRouterRestoreSnapshot(history, "snap-a", false)
	if err != nil {
		t.Fatalf("select by id: %v", err)
	}
	if got.ID != "snap-a" {
		t.Fatalf("expected snapshot snap-a, got %q", got.ID)
	}
}

func TestRouterSyncSummaryFromResult_IncludesResourceBreakdownAndDeleteCandidates(t *testing.T) {
	result := &routercontracts.SyncResult{
		DryRun: true,
		Actions: []routercontracts.SyncAction{
			{Resource: "dns_record", Action: "create", Name: "svc.example.com"},
			{Resource: "dns_record", Action: "no-op", Name: "svc.example.com"},
			{Resource: "lb_pool", Action: "update", Name: "svc-dev-pool"},
			{Resource: "lb_pool", Action: "delete-candidate", Name: "svc-dev-pool-old"},
		},
	}
	summary := RouterSyncSummaryFromResult(result)
	if summary.Create != 1 || summary.Update != 1 || summary.Noop != 1 || summary.DeleteCandidate != 1 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if !summary.DriftDetected {
		t.Fatal("expected drift detected when create/update/delete-candidate actions exist")
	}
	if summary.ByResource["dns_record"].Create != 1 || summary.ByResource["dns_record"].Noop != 1 {
		t.Fatalf("unexpected dns_record resource summary: %#v", summary.ByResource["dns_record"])
	}
	if summary.ByResource["lb_pool"].Update != 1 || summary.ByResource["lb_pool"].DeleteCandidate != 1 {
		t.Fatalf("unexpected lb_pool resource summary: %#v", summary.ByResource["lb_pool"])
	}
}

func TestAnalyzeRouterSyncHistory_ComputesTrendAndTotals(t *testing.T) {
	history := []statecore.RouterSyncSnapshot{
		{
			ID:     "1",
			DryRun: false,
			Actions: []statecore.RouterSyncAction{
				{Resource: "dns_record", Action: "create"},
				{Resource: "lb_pool", Action: "update"},
				{Resource: "lb_monitor", Action: "update"},
			},
		},
		{
			ID:     "2",
			DryRun: false,
			Actions: []statecore.RouterSyncAction{
				{Resource: "dns_record", Action: "update"},
				{Resource: "lb_pool", Action: "update"},
			},
		},
		{
			ID:     "3",
			DryRun: true,
			Actions: []statecore.RouterSyncAction{
				{Resource: "dns_record", Action: "no-op"},
			},
		},
		{
			ID:        "4",
			Operation: "router-sync-123",
			Trigger:   "cli-router",
			DryRun:    false,
			CreatedAt: "2026-03-28T10:00:00Z",
			Actions: []statecore.RouterSyncAction{
				{Resource: "dns_record", Action: "update"},
			},
		},
	}

	analytics := AnalyzeRouterSyncHistory(history, 2)
	if analytics.Total.Snapshots != 4 || analytics.Total.Applied != 3 || analytics.Total.DryRun != 1 {
		t.Fatalf("unexpected total window counters: %#v", analytics.Total)
	}
	if analytics.Total.Create != 1 || analytics.Total.Update != 5 || analytics.Total.Noop != 1 {
		t.Fatalf("unexpected total action counters: %#v", analytics.Total)
	}
	if analytics.Trend != "improving" {
		t.Fatalf("expected improving trend, got %q", analytics.Trend)
	}
	if analytics.LastSnapshotID != "4" {
		t.Fatalf("expected last snapshot id=4, got %q", analytics.LastSnapshotID)
	}
	if analytics.LastOperation != "router-sync-123" || analytics.LastTrigger != "cli-router" {
		t.Fatalf("unexpected last operation metadata: operation=%q trigger=%q", analytics.LastOperation, analytics.LastTrigger)
	}
	if analytics.ByResource["dns_record"].Update != 2 {
		t.Fatalf("expected dns_record update aggregation, got %#v", analytics.ByResource["dns_record"])
	}
}

func TestAnalyzeRouterSyncHistory_InsufficientDataWhenNoPreviousWindow(t *testing.T) {
	history := []statecore.RouterSyncSnapshot{
		{
			ID: "1",
			Actions: []statecore.RouterSyncAction{
				{Resource: "dns_record", Action: "create"},
			},
		},
	}
	analytics := AnalyzeRouterSyncHistory(history, 3)
	if analytics.Trend != "insufficient-data" {
		t.Fatalf("expected insufficient-data trend, got %q", analytics.Trend)
	}
}
