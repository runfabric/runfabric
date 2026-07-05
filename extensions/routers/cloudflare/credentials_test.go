package cloudflare

import (
	"os"
	"testing"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func clear(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestTokenFallsBackToProviderKey(t *testing.T) {
	clear(t, "RUNFABRIC_ROUTER_API_TOKEN", "RUNFABRIC_ROUTER_API_TOKEN_FILE")
	t.Setenv("CLOUDFLARE_API_TOKEN", "provider-token")
	if got := resolveCloudflareAPIToken(); got != "provider-token" {
		t.Fatalf("token = %q, want provider-key fallback", got)
	}
}

func TestRouterTokenWinsOverProviderKey(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "router-token")
	t.Setenv("CLOUDFLARE_API_TOKEN", "provider-token")
	if got := resolveCloudflareAPIToken(); got != "router-token" {
		t.Fatalf("token = %q, router-specific value must win", got)
	}
}

func TestZoneAndAccountFallBackToProviderKeys(t *testing.T) {
	clear(t, "RUNFABRIC_ROUTER_ZONE_ID", "RUNFABRIC_ROUTER_ACCOUNT_ID")
	t.Setenv("CLOUDFLARE_ZONE_ID", "zone-from-provider")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "account-from-provider")

	if got := sdkprovider.ResolveVar(CredentialVars, "RUNFABRIC_ROUTER_ZONE_ID"); got != "zone-from-provider" {
		t.Fatalf("zone = %q", got)
	}
	if got := sdkprovider.ResolveVar(CredentialVars, "RUNFABRIC_ROUTER_ACCOUNT_ID"); got != "account-from-provider" {
		t.Fatalf("account = %q", got)
	}
	// Router-specific env wins.
	t.Setenv("RUNFABRIC_ROUTER_ZONE_ID", "zone-router")
	if got := sdkprovider.ResolveVar(CredentialVars, "RUNFABRIC_ROUTER_ZONE_ID"); got != "zone-router" {
		t.Fatalf("zone = %q, router-specific value must win", got)
	}
}
