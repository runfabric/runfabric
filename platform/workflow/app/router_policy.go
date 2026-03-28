package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/core/policy/secrets"
)

type RouterDNSSyncPolicy struct {
	AutoApply           bool              `json:"autoApply"`
	DryRun              bool              `json:"dryRun"`
	AllowProdSync       bool              `json:"allowProdSync"`
	EnforceStageRollout bool              `json:"enforceStageRollout"`
	RequireReason       bool              `json:"requireReason"`
	ReasonEnv           string            `json:"reasonEnv"`
	ApprovalEnvByStage  map[string]string `json:"approvalEnvByStage"`
	ZoneIDEnv           string            `json:"zoneIDEnv"`
	AccountIDEnv        string            `json:"accountIDEnv"`
	APITokenEnv         string            `json:"apiTokenEnv"`
	APITokenFileEnv     string            `json:"apiTokenFileEnv"`
	APITokenSecretRef   string            `json:"apiTokenSecretRef"`
	MutationPolicy      RouterMutationPolicy
	CredentialPolicy    RouterCredentialPolicy
}

type RouterMutationPolicy struct {
	Enabled                     bool     `json:"enabled"`
	ApprovalEnv                 string   `json:"approvalEnv"`
	RiskyResources              []string `json:"riskyResources,omitempty"`
	MaxMutationsWithoutApproval int      `json:"maxMutationsWithoutApproval"`
}

type RouterCredentialPolicy struct {
	Enabled             bool   `json:"enabled"`
	RequireAttestation  bool   `json:"requireAttestation"`
	AttestationEnv      string `json:"attestationEnv"`
	IssuedAtEnv         string `json:"issuedAtEnv"`
	ExpiresAtEnv        string `json:"expiresAtEnv"`
	MaxTTLSeconds       int    `json:"maxTTLSeconds"`
	MinRemainingSeconds int    `json:"minRemainingSeconds"`
}

func RouterDNSSyncPolicyForStage(cfg *config.Config, stage string) RouterDNSSyncPolicy {
	policy := RouterDNSSyncPolicy{
		AutoApply:           false,
		DryRun:              false,
		AllowProdSync:       false,
		EnforceStageRollout: false,
		RequireReason:       false,
		ReasonEnv:           "RUNFABRIC_DNS_SYNC_REASON",
		ApprovalEnvByStage: map[string]string{
			"staging": "RUNFABRIC_DNS_SYNC_DEV_APPROVED",
			"prod":    "RUNFABRIC_DNS_SYNC_STAGING_APPROVED",
		},
		ZoneIDEnv:         "RUNFABRIC_ROUTER_ZONE_ID",
		AccountIDEnv:      "RUNFABRIC_ROUTER_ACCOUNT_ID",
		APITokenEnv:       "RUNFABRIC_ROUTER_API_TOKEN",
		APITokenFileEnv:   "RUNFABRIC_ROUTER_API_TOKEN_FILE",
		APITokenSecretRef: "",
		MutationPolicy: RouterMutationPolicy{
			Enabled:                     false,
			ApprovalEnv:                 "RUNFABRIC_DNS_SYNC_RISK_APPROVED",
			RiskyResources:              []string{"lb_monitor", "lb_pool", "load_balancer"},
			MaxMutationsWithoutApproval: 0,
		},
		CredentialPolicy: RouterCredentialPolicy{
			Enabled:             false,
			RequireAttestation:  true,
			AttestationEnv:      "RUNFABRIC_ROUTER_TOKEN_ATTESTED",
			IssuedAtEnv:         "RUNFABRIC_ROUTER_TOKEN_ISSUED_AT",
			ExpiresAtEnv:        "RUNFABRIC_ROUTER_TOKEN_EXPIRES_AT",
			MaxTTLSeconds:       3600,
			MinRemainingSeconds: 120,
		},
	}
	if cfg == nil || cfg.Extensions == nil {
		return policy
	}
	raw, ok := cfg.Extensions["router"]
	if !ok || raw == nil {
		return policy
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return policy
	}
	policy.applyObject(obj, normalizeStage(stage))
	return policy
}

func (p *RouterDNSSyncPolicy) applyObject(obj map[string]any, stage string) {
	if obj == nil {
		return
	}
	if v, ok := parseBool(obj["autoApply"]); ok {
		p.AutoApply = v
	}
	if autoObj, ok := asMap(obj["autoApply"]); ok {
		p.applyAutoObject(autoObj, stage)
	}
	if v, ok := parseBool(obj["dryRun"]); ok {
		p.DryRun = v
	}
	if v, ok := parseBool(obj["allowProdSync"]); ok {
		p.AllowProdSync = v
	}
	if v, ok := parseBool(obj["enforceStageRollout"]); ok {
		p.EnforceStageRollout = v
	}
	if v, ok := parseBool(obj["requireReason"]); ok {
		p.RequireReason = v
	}
	if v, ok := parseString(obj["reasonEnv"]); ok {
		p.ReasonEnv = v
	}
	if approvals, ok := asMap(obj["approvalEnvByStage"]); ok {
		for k, rawV := range approvals {
			if v, ok := parseString(rawV); ok {
				p.ApprovalEnvByStage[normalizeStage(k)] = v
			}
		}
	}
	if credentials, ok := asMap(obj["credentials"]); ok {
		if v, ok := parseString(credentials["zoneIDEnv"]); ok {
			p.ZoneIDEnv = v
		}
		if v, ok := parseString(credentials["accountIDEnv"]); ok {
			p.AccountIDEnv = v
		}
		if v, ok := parseString(credentials["apiTokenEnv"]); ok {
			p.APITokenEnv = v
		}
		if v, ok := parseString(credentials["apiTokenFileEnv"]); ok {
			p.APITokenFileEnv = v
		}
		if v, ok := parseString(credentials["apiTokenSecretRef"]); ok {
			p.APITokenSecretRef = v
		}
	}
	if mutationPolicy, ok := asMap(obj["mutationPolicy"]); ok {
		p.MutationPolicy.Enabled = true
		p.applyMutationPolicy(mutationPolicy)
	}
	if credentialPolicy, ok := asMap(obj["credentialPolicy"]); ok {
		p.CredentialPolicy.Enabled = true
		p.applyCredentialPolicy(credentialPolicy)
	}

	if stages, ok := asMap(obj["stages"]); ok {
		if stageObj, ok := asMap(stages[stage]); ok {
			p.applyObject(stageObj, stage)
		}
	}
}

func (p *RouterDNSSyncPolicy) applyMutationPolicy(obj map[string]any) {
	if obj == nil {
		return
	}
	if v, ok := parseBool(obj["enabled"]); ok {
		p.MutationPolicy.Enabled = v
	}
	if v, ok := parseString(obj["approvalEnv"]); ok {
		p.MutationPolicy.ApprovalEnv = v
	}
	if v, ok := parseStringList(obj["riskyResources"]); ok {
		p.MutationPolicy.RiskyResources = v
	}
	if v, ok := parseInt(obj["maxMutationsWithoutApproval"]); ok {
		p.MutationPolicy.MaxMutationsWithoutApproval = v
	}
}

func (p *RouterDNSSyncPolicy) applyCredentialPolicy(obj map[string]any) {
	if obj == nil {
		return
	}
	if v, ok := parseBool(obj["enabled"]); ok {
		p.CredentialPolicy.Enabled = v
	}
	if v, ok := parseBool(obj["requireAttestation"]); ok {
		p.CredentialPolicy.RequireAttestation = v
	}
	if v, ok := parseString(obj["attestationEnv"]); ok {
		p.CredentialPolicy.AttestationEnv = v
	}
	if v, ok := parseString(obj["issuedAtEnv"]); ok {
		p.CredentialPolicy.IssuedAtEnv = v
	}
	if v, ok := parseString(obj["expiresAtEnv"]); ok {
		p.CredentialPolicy.ExpiresAtEnv = v
	}
	if v, ok := parseInt(obj["maxTTLSeconds"]); ok {
		p.CredentialPolicy.MaxTTLSeconds = v
	}
	if v, ok := parseInt(obj["minRemainingSeconds"]); ok {
		p.CredentialPolicy.MinRemainingSeconds = v
	}
}

func (p *RouterDNSSyncPolicy) applyAutoObject(autoObj map[string]any, stage string) {
	if autoObj == nil {
		return
	}
	enabled := true
	if v, ok := parseBool(autoObj["enabled"]); ok {
		enabled = v
	}
	if v, ok := parseBool(autoObj["dryRun"]); ok {
		p.DryRun = v
	}
	if v, ok := parseBool(autoObj["allowProdSync"]); ok {
		p.AllowProdSync = v
	}
	if v, ok := parseBool(autoObj["enforceStageRollout"]); ok {
		p.EnforceStageRollout = v
	}
	if v, ok := parseStageList(autoObj["stages"]); ok {
		p.AutoApply = enabled && stageAllowed(v, stage)
		return
	}
	p.AutoApply = enabled
}

func normalizeStage(stage string) string {
	s := strings.ToLower(strings.TrimSpace(stage))
	if s == "" {
		return "dev"
	}
	return s
}

func stageAllowed(stages []string, stage string) bool {
	if len(stages) == 0 {
		return true
	}
	stage = normalizeStage(stage)
	for _, s := range stages {
		if normalizeStage(s) == stage {
			return true
		}
	}
	return false
}

func parseStageList(v any) ([]string, bool) {
	return parseStringList(v)
}

func parseStringList(v any) ([]string, bool) {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := parseString(item); ok {
				out = append(out, s)
			}
		}
		return out, true
	case []string:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := parseString(item); ok {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func parseInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int32:
		return int(t), true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		var out int
		if _, err := fmt.Sscanf(s, "%d", &out); err != nil {
			return 0, false
		}
		return out, true
	default:
		return 0, false
	}
}

func parseBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		switch s {
		case "1", "true", "yes":
			return true, true
		case "0", "false", "no":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func parseString(v any) (string, bool) {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return "", false
	}
	return s, true
}

func asMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return m, true
}

// ResolveRouterAPITokenSecretRef resolves a router API token from a secret reference.
// Supported values:
// - KEY (resolved as ${secret:KEY})
// - ${secret:KEY}
// - secret://KEY
func ResolveRouterAPITokenSecretRef(cfg *config.Config, secretRef string) (string, error) {
	ref := strings.TrimSpace(secretRef)
	if ref == "" {
		return "", nil
	}
	input := ref
	switch {
	case strings.Contains(ref, "${secret:"):
		// direct placeholder
	case strings.HasPrefix(ref, "secret://"):
		key := strings.TrimSpace(strings.TrimPrefix(ref, "secret://"))
		if key == "" {
			return "", fmt.Errorf("router API token secret ref %q is invalid", secretRef)
		}
		input = "${secret:" + key + "}"
	default:
		input = "${secret:" + ref + "}"
	}

	var configSecrets map[string]string
	if cfg != nil {
		configSecrets = cfg.Secrets
	}
	resolved, err := secrets.ResolveString(input, configSecrets, os.LookupEnv)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(resolved)
	if token == "" {
		return "", fmt.Errorf("router API token secret ref %q resolved to an empty value", secretRef)
	}
	return token, nil
}
