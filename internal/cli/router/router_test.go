package router

import (
	"strings"
	"testing"

	"github.com/runfabric/runfabric/internal/app"
	modelconfig "github.com/runfabric/runfabric/platform/core/model/config"
)

func TestEnforceDNSSyncStageGate_ProdRequiresAllowFlag(t *testing.T) {
	err := enforceDNSSyncStageGate("prod", false, false)
	if err == nil {
		t.Fatal("expected error for prod without --allow-prod-dns-sync")
	}
	if !strings.Contains(err.Error(), "--allow-prod-dns-sync") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_NoRolloutEnforcement_AllowsNonProd(t *testing.T) {
	err := enforceDNSSyncStageGate("staging", false, false)
	if err != nil {
		t.Fatalf("expected no error when rollout enforcement disabled, got: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_StagingRequiresDevApproval(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_DEV_APPROVED", "")
	err := enforceDNSSyncStageGate("staging", false, true)
	if err == nil {
		t.Fatal("expected staging approval error")
	}
	if !strings.Contains(err.Error(), "RUNFABRIC_DNS_SYNC_DEV_APPROVED=true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_StagingApproved(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_DEV_APPROVED", "true")
	err := enforceDNSSyncStageGate("staging", false, true)
	if err != nil {
		t.Fatalf("expected staging approval to pass, got: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_ProdRequiresStagingApproval(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_STAGING_APPROVED", "")
	err := enforceDNSSyncStageGate("prod", true, true)
	if err == nil {
		t.Fatal("expected prod staging approval error")
	}
	if !strings.Contains(err.Error(), "RUNFABRIC_DNS_SYNC_STAGING_APPROVED=true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_ProdApproved(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_STAGING_APPROVED", "TrUe")
	err := enforceDNSSyncStageGate("prod", true, true)
	if err != nil {
		t.Fatalf("expected prod approval to pass, got: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_UnknownStage(t *testing.T) {
	err := enforceDNSSyncStageGate("qa", true, true)
	if err == nil {
		t.Fatal("expected unsupported stage error")
	}
	if !strings.Contains(err.Error(), "unsupported stage") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceDNSSyncStageGate_EmptyStageTreatedAsDev(t *testing.T) {
	err := enforceDNSSyncStageGate("", false, true)
	if err != nil {
		t.Fatalf("expected empty stage to pass as dev, got: %v", err)
	}
}

func TestSelectedRouterPlugin_DefaultsWhenUnset(t *testing.T) {
	id := app.SelectedRouterPlugin(&modelconfig.Config{})
	if strings.TrimSpace(id) == "" {
		t.Fatal("expected non-empty default router plugin id")
	}
}

func TestSelectedRouterPlugin_FromExtensions_NormalizesToLowerTrim(t *testing.T) {
	id := app.SelectedRouterPlugin(&modelconfig.Config{Extensions: map[string]any{"routerPlugin": "  Custom-Router  "}})
	if id != "custom-router" {
		t.Fatalf("expected normalized router plugin id custom-router, got %q", id)
	}
}
