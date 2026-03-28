package app

import (
	"testing"

	modelconfig "github.com/runfabric/runfabric/platform/core/model/config"
)

func TestRouterDNSSyncPolicyForStage_Defaults(t *testing.T) {
	p := RouterDNSSyncPolicyForStage(&modelconfig.Config{}, "dev")
	if p.AutoApply {
		t.Fatal("expected auto apply disabled by default")
	}
	if p.ZoneIDEnv != "RUNFABRIC_ROUTER_ZONE_ID" {
		t.Fatalf("unexpected zone env default: %s", p.ZoneIDEnv)
	}
	if p.AccountIDEnv != "RUNFABRIC_ROUTER_ACCOUNT_ID" {
		t.Fatalf("unexpected account env default: %s", p.AccountIDEnv)
	}
	if p.APITokenEnv != "RUNFABRIC_ROUTER_API_TOKEN" {
		t.Fatalf("unexpected api token env default: %s", p.APITokenEnv)
	}
	if p.MutationPolicy.Enabled {
		t.Fatal("expected mutation policy disabled by default")
	}
	if p.MutationPolicy.ApprovalEnv != "RUNFABRIC_DNS_SYNC_RISK_APPROVED" {
		t.Fatalf("unexpected mutation approval env default: %s", p.MutationPolicy.ApprovalEnv)
	}
}

func TestRouterDNSSyncPolicyForStage_AutoApplyStagesAndOverrides(t *testing.T) {
	cfg := &modelconfig.Config{
		Extensions: map[string]any{
			"router": map[string]any{
				"autoApply": map[string]any{
					"enabled":             true,
					"stages":              []any{"staging", "prod"},
					"enforceStageRollout": true,
				},
				"credentials": map[string]any{
					"zoneIDEnv":         "CF_ZONE_ENV",
					"accountIDEnv":      "CF_ACCOUNT_ENV",
					"apiTokenEnv":       "CF_TOKEN_ENV",
					"apiTokenSecretRef": "router_api_token",
				},
				"mutationPolicy": map[string]any{
					"enabled":                     true,
					"approvalEnv":                 "DNS_RISK_APPROVED",
					"riskyResources":              []any{"load_balancer", "dns_record"},
					"maxMutationsWithoutApproval": 2,
				},
				"credentialPolicy": map[string]any{
					"enabled":             true,
					"requireAttestation":  true,
					"attestationEnv":      "ROUTER_TOKEN_ATTESTED",
					"issuedAtEnv":         "ROUTER_TOKEN_ISSUED_AT",
					"expiresAtEnv":        "ROUTER_TOKEN_EXPIRES_AT",
					"maxTTLSeconds":       1800,
					"minRemainingSeconds": 90,
				},
				"approvalEnvByStage": map[string]any{
					"staging": "CUSTOM_STAGING_APPROVAL",
				},
				"stages": map[string]any{
					"prod": map[string]any{
						"dryRun":        true,
						"requireReason": true,
						"reasonEnv":     "DNS_CHANGE_REASON",
					},
				},
			},
		},
	}

	stagingPolicy := RouterDNSSyncPolicyForStage(cfg, "staging")
	if !stagingPolicy.AutoApply {
		t.Fatal("expected staging auto apply enabled")
	}
	if stagingPolicy.DryRun {
		t.Fatal("expected staging dry run false")
	}
	if !stagingPolicy.EnforceStageRollout {
		t.Fatal("expected rollout enforcement enabled")
	}
	if stagingPolicy.ApprovalEnvByStage["staging"] != "CUSTOM_STAGING_APPROVAL" {
		t.Fatalf("expected custom staging approval env, got %q", stagingPolicy.ApprovalEnvByStage["staging"])
	}
	if stagingPolicy.ZoneIDEnv != "CF_ZONE_ENV" || stagingPolicy.AccountIDEnv != "CF_ACCOUNT_ENV" || stagingPolicy.APITokenEnv != "CF_TOKEN_ENV" {
		t.Fatalf(
			"expected custom credential envs, got zone=%q account=%q token=%q",
			stagingPolicy.ZoneIDEnv,
			stagingPolicy.AccountIDEnv,
			stagingPolicy.APITokenEnv,
		)
	}
	if stagingPolicy.APITokenSecretRef != "router_api_token" {
		t.Fatalf("expected custom secret ref, got %q", stagingPolicy.APITokenSecretRef)
	}
	if !stagingPolicy.MutationPolicy.Enabled {
		t.Fatal("expected mutation policy enabled")
	}
	if stagingPolicy.MutationPolicy.ApprovalEnv != "DNS_RISK_APPROVED" {
		t.Fatalf("unexpected mutation approval env: %q", stagingPolicy.MutationPolicy.ApprovalEnv)
	}
	if stagingPolicy.MutationPolicy.MaxMutationsWithoutApproval != 2 {
		t.Fatalf("unexpected mutation max threshold: %d", stagingPolicy.MutationPolicy.MaxMutationsWithoutApproval)
	}
	if !stagingPolicy.CredentialPolicy.Enabled {
		t.Fatal("expected credential policy enabled")
	}
	if stagingPolicy.CredentialPolicy.AttestationEnv != "ROUTER_TOKEN_ATTESTED" {
		t.Fatalf("unexpected attestation env: %q", stagingPolicy.CredentialPolicy.AttestationEnv)
	}
	if stagingPolicy.CredentialPolicy.MaxTTLSeconds != 1800 || stagingPolicy.CredentialPolicy.MinRemainingSeconds != 90 {
		t.Fatalf("unexpected credential policy thresholds: ttl=%d remaining=%d", stagingPolicy.CredentialPolicy.MaxTTLSeconds, stagingPolicy.CredentialPolicy.MinRemainingSeconds)
	}

	devPolicy := RouterDNSSyncPolicyForStage(cfg, "dev")
	if devPolicy.AutoApply {
		t.Fatal("expected dev auto apply disabled because stage list excludes dev")
	}

	prodPolicy := RouterDNSSyncPolicyForStage(cfg, "prod")
	if !prodPolicy.AutoApply {
		t.Fatal("expected prod auto apply enabled")
	}
	if !prodPolicy.DryRun {
		t.Fatal("expected prod dry run override enabled")
	}
	if !prodPolicy.RequireReason || prodPolicy.ReasonEnv != "DNS_CHANGE_REASON" {
		t.Fatalf("expected prod reason enforcement override, got require=%v env=%q", prodPolicy.RequireReason, prodPolicy.ReasonEnv)
	}
}

func TestResolveRouterAPITokenSecretRef_ResolvesFromConfigSecrets(t *testing.T) {
	t.Setenv("CF_ROUTER_TOKEN", "token-from-env")
	cfg := &modelconfig.Config{
		Secrets: map[string]string{
			"router_api_token": "secret://CF_ROUTER_TOKEN",
		},
	}
	token, err := ResolveRouterAPITokenSecretRef(cfg, "router_api_token")
	if err != nil {
		t.Fatalf("resolve secret ref failed: %v", err)
	}
	if token != "token-from-env" {
		t.Fatalf("unexpected token value: %q", token)
	}
}

func TestResolveRouterAPITokenSecretRef_MissingReferenceFails(t *testing.T) {
	cfg := &modelconfig.Config{}
	if _, err := ResolveRouterAPITokenSecretRef(cfg, "MISSING_ROUTER_TOKEN"); err == nil {
		t.Fatal("expected missing secret reference error")
	}
}
