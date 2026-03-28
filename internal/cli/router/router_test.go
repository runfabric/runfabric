package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	routercontracts "github.com/runfabric/runfabric/platform/core/contracts/router"
	modelconfig "github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/workflow/app"
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

func TestEnforceDNSSyncStageGateWithPolicy_CustomApprovalEnv(t *testing.T) {
	t.Setenv("CUSTOM_STAGING_APPROVAL", "true")
	err := enforceDNSSyncStageGateWithPolicy("staging", false, true, map[string]string{"staging": "CUSTOM_STAGING_APPROVAL"}, false, "")
	if err != nil {
		t.Fatalf("expected custom approval env to pass, got: %v", err)
	}
}

func TestEnforceDNSSyncStageGateWithPolicy_RequiresReason(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_REASON", "")
	err := enforceDNSSyncStageGateWithPolicy("dev", false, false, nil, true, "")
	if err == nil {
		t.Fatal("expected reason enforcement error")
	}
	if !strings.Contains(err.Error(), "RUNFABRIC_DNS_SYNC_REASON") {
		t.Fatalf("unexpected reason error: %v", err)
	}
}

func TestResolveDNSProviderIDs_UsesCustomEnvNames(t *testing.T) {
	t.Setenv("CUSTOM_ZONE_ENV", "zone-custom")
	t.Setenv("CUSTOM_ACCOUNT_ENV", "account-custom")
	zoneID, accountID := resolveDNSProviderIDs("", "", "CUSTOM_ZONE_ENV", "CUSTOM_ACCOUNT_ENV")
	if zoneID != "zone-custom" || accountID != "account-custom" {
		t.Fatalf("unexpected provider ids from custom envs: zone=%q account=%q", zoneID, accountID)
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

func TestPrimeRouterAPIToken_FromCustomEnv(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CUSTOM_ROUTER_TOKEN_ENV", "token-from-custom-env")
	policy := app.RouterDNSSyncPolicy{APITokenEnv: "CUSTOM_ROUTER_TOKEN_ENV"}
	if err := primeRouterAPIToken(nil, policy); err != nil {
		t.Fatalf("primeRouterAPIToken returned error: %v", err)
	}
	if got := strings.TrimSpace(os.Getenv("RUNFABRIC_ROUTER_API_TOKEN")); got != "token-from-custom-env" {
		t.Fatalf("unexpected primed token value: %q", got)
	}
}

func TestPrimeRouterAPIToken_FromTokenFileEnv(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "router-token.txt")
	if err := os.WriteFile(tokenPath, []byte("token-from-file\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	t.Setenv("CUSTOM_ROUTER_TOKEN_FILE_ENV", tokenPath)
	policy := app.RouterDNSSyncPolicy{APITokenFileEnv: "CUSTOM_ROUTER_TOKEN_FILE_ENV"}
	if err := primeRouterAPIToken(nil, policy); err != nil {
		t.Fatalf("primeRouterAPIToken returned error: %v", err)
	}
	if got := strings.TrimSpace(os.Getenv("RUNFABRIC_ROUTER_API_TOKEN")); got != "token-from-file" {
		t.Fatalf("unexpected primed token value from file: %q", got)
	}
}

func TestPrimeRouterAPIToken_FromSecretRef(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CF_ROUTER_TOKEN", "token-from-secret-ref")
	ctx := &app.AppContext{
		Config: &modelconfig.Config{
			Secrets: map[string]string{
				"router_api_token": "secret://CF_ROUTER_TOKEN",
			},
		},
	}
	policy := app.RouterDNSSyncPolicy{APITokenSecretRef: "router_api_token"}
	if err := primeRouterAPIToken(ctx, policy); err != nil {
		t.Fatalf("primeRouterAPIToken returned error: %v", err)
	}
	if got := strings.TrimSpace(os.Getenv("RUNFABRIC_ROUTER_API_TOKEN")); got != "token-from-secret-ref" {
		t.Fatalf("unexpected primed token value from secret ref: %q", got)
	}
}

func TestEnforceRouterMutationPolicy_RiskyMutationRequiresApproval(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_RISK_APPROVED", "")
	policy := app.RouterMutationPolicy{
		Enabled:        true,
		ApprovalEnv:    "RUNFABRIC_DNS_SYNC_RISK_APPROVED",
		RiskyResources: []string{"load_balancer"},
	}
	preview := &routercontracts.SyncResult{
		DryRun: true,
		Actions: []routercontracts.SyncAction{
			{Resource: "load_balancer", Action: "update", Name: "svc.example.com"},
		},
	}
	err := enforceRouterMutationPolicy(policy, preview)
	if err == nil {
		t.Fatal("expected mutation policy enforcement error for risky mutation without approval")
	}
	if !strings.Contains(err.Error(), "RUNFABRIC_DNS_SYNC_RISK_APPROVED=true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceRouterMutationPolicy_MutationThresholdRequiresApproval(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_RISK_APPROVED", "")
	policy := app.RouterMutationPolicy{
		Enabled:                     true,
		ApprovalEnv:                 "RUNFABRIC_DNS_SYNC_RISK_APPROVED",
		MaxMutationsWithoutApproval: 1,
	}
	preview := &routercontracts.SyncResult{
		DryRun: true,
		Actions: []routercontracts.SyncAction{
			{Resource: "dns_record", Action: "create", Name: "svc.example.com"},
			{Resource: "dns_record", Action: "update", Name: "svc.example.com"},
		},
	}
	err := enforceRouterMutationPolicy(policy, preview)
	if err == nil {
		t.Fatal("expected mutation policy threshold error")
	}
	if !strings.Contains(err.Error(), "maxMutationsWithoutApproval=1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceRouterMutationPolicy_ApprovedPasses(t *testing.T) {
	t.Setenv("RUNFABRIC_DNS_SYNC_RISK_APPROVED", "true")
	policy := app.RouterMutationPolicy{
		Enabled:        true,
		ApprovalEnv:    "RUNFABRIC_DNS_SYNC_RISK_APPROVED",
		RiskyResources: []string{"load_balancer"},
	}
	preview := &routercontracts.SyncResult{
		DryRun: true,
		Actions: []routercontracts.SyncAction{
			{Resource: "load_balancer", Action: "update", Name: "svc.example.com"},
		},
	}
	if err := enforceRouterMutationPolicy(policy, preview); err != nil {
		t.Fatalf("expected policy enforcement to pass when approved, got: %v", err)
	}
}

func TestEnforceRouterCredentialPolicy_RequiresAttestation(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_ATTESTED", "")
	policy := app.RouterCredentialPolicy{
		Enabled:            true,
		RequireAttestation: true,
		AttestationEnv:     "RUNFABRIC_ROUTER_TOKEN_ATTESTED",
	}
	if err := enforceRouterCredentialPolicy(policy, time.Now().UTC()); err == nil {
		t.Fatal("expected attestation enforcement error")
	}
}

func TestEnforceRouterCredentialPolicy_ValidShortLivedTokenPasses(t *testing.T) {
	now := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)
	issued := now.Add(-5 * time.Minute)
	expires := now.Add(25 * time.Minute)
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_ATTESTED", "true")
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_ISSUED_AT", issued.Format(time.RFC3339))
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT", expires.Format(time.RFC3339))
	policy := app.RouterCredentialPolicy{
		Enabled:             true,
		RequireAttestation:  true,
		AttestationEnv:      "RUNFABRIC_ROUTER_TOKEN_ATTESTED",
		IssuedAtEnv:         "RUNFABRIC_ROUTER_TOKEN_ISSUED_AT",
		ExpiresAtEnv:        "RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT",
		MaxTTLSeconds:       3600,
		MinRemainingSeconds: 60,
	}
	if err := enforceRouterCredentialPolicy(policy, now); err != nil {
		t.Fatalf("expected credential policy to pass, got: %v", err)
	}
}

func TestEnforceRouterCredentialPolicy_MaxTTLRejected(t *testing.T) {
	now := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)
	issued := now.Add(-2 * time.Hour)
	expires := now.Add(2 * time.Hour)
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_ATTESTED", "true")
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_ISSUED_AT", issued.Format(time.RFC3339))
	t.Setenv("RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT", expires.Format(time.RFC3339))
	policy := app.RouterCredentialPolicy{
		Enabled:             true,
		RequireAttestation:  true,
		AttestationEnv:      "RUNFABRIC_ROUTER_TOKEN_ATTESTED",
		IssuedAtEnv:         "RUNFABRIC_ROUTER_TOKEN_ISSUED_AT",
		ExpiresAtEnv:        "RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT",
		MaxTTLSeconds:       1800,
		MinRemainingSeconds: 60,
	}
	err := enforceRouterCredentialPolicy(policy, now)
	if err == nil {
		t.Fatal("expected credential policy max TTL rejection")
	}
	if !strings.Contains(err.Error(), "max allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
