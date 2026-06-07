package runtime

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func mcpRuntimeWithProviderPolicy(allowServers []string, defaultDeny bool, pp config.MCPProviderPolicyRule, region string) *MCPRuntime {
	r := NewMCPRuntime(
		NoopMCPClient{},
		config.MCPIntegrationsConfig{Servers: map[string]config.MCPServerConfig{"db": {URL: "http://db"}}},
		config.MCPPolicyConfig{
			Allow:       config.MCPPolicyRuleSet{Servers: allowServers},
			DefaultDeny: defaultDeny,
			Providers:   map[string]config.MCPProviderPolicyRule{"aws-lambda": pp},
		},
	)
	r.Provider = "aws-lambda"
	r.ActiveRegion = region
	return r
}

// TestEnsureAllowed_ExplicitAllowDoesNotBypassProviderRegionDeny guards the
// regression where provider region/auth enforcement only ran on the
// default-allow path, so an explicit allow rule let a denied-region MCP call
// through.
func TestEnsureAllowed_ExplicitAllowDoesNotBypassProviderRegionDeny(t *testing.T) {
	r := mcpRuntimeWithProviderPolicy(
		[]string{"db"}, // explicit allow for server "db"
		false,
		config.MCPProviderPolicyRule{DenyRegions: []string{"us-east-1"}},
		"us-east-1", // active region is in the provider deny list
	)

	err := r.ensureAllowed("tool", "db", "read", map[string]any{})
	if err == nil {
		t.Fatal("expected provider region deny to apply despite the explicit allow rule")
	}
}

// TestEnsureAllowed_CrossRegionDenyAppliesUnderExplicitAllow covers the
// DenyCrossRegion path with an explicit allow rule present.
func TestEnsureAllowed_CrossRegionDenyAppliesUnderExplicitAllow(t *testing.T) {
	r := mcpRuntimeWithProviderPolicy(
		[]string{"db"},
		false,
		config.MCPProviderPolicyRule{DenyCrossRegion: true, RequiredRegion: "us-west-2"},
		"eu-west-1",
	)

	if err := r.ensureAllowed("tool", "db", "read", map[string]any{}); err == nil {
		t.Fatal("expected cross-region deny to apply despite the explicit allow rule")
	}
}

// TestEnsureAllowed_AllowedWhenRegionPermitted confirms the explicit allow still
// succeeds when the provider region constraints are satisfied.
func TestEnsureAllowed_AllowedWhenRegionPermitted(t *testing.T) {
	r := mcpRuntimeWithProviderPolicy(
		[]string{"db"},
		false,
		config.MCPProviderPolicyRule{DenyCrossRegion: true, RequiredRegion: "us-east-1"},
		"us-east-1",
	)

	if err := r.ensureAllowed("tool", "db", "read", map[string]any{}); err != nil {
		t.Fatalf("expected allow when region matches, got %v", err)
	}
}

func TestWildcardMatch_GlobSemantics(t *testing.T) {
	cases := []struct {
		pattern, value string
		want           bool
	}{
		{"*", "anything", true},
		{"db.read", "db.read", true},
		{"db.read", "db.write", false},
		{"db.read*", "db.read", true},
		{"db.read*", "db.read-then-delete", true},
		{"*-admin", "backup-admin", true}, // leading * (was a no-op before)
		{"*-admin", "backup-reader", false},
		{"db.*.delete", "db.users.delete", true}, // interior *
		{"db.*.delete", "db.users.read", false},
		{"a*b*c", "axxbyyc", true},
		{"a*b*c", "axxc", false},
	}
	for _, c := range cases {
		if got := wildcardMatch(c.pattern, c.value); got != c.want {
			t.Errorf("wildcardMatch(%q, %q) = %v, want %v", c.pattern, c.value, got, c.want)
		}
	}
}
