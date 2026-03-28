package cloudflare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCloudflareAPIToken_PrefersRouterEnv(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "router-token")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cloudflare-token")
	got := resolveCloudflareAPIToken()
	if got != "router-token" {
		t.Fatalf("expected RUNFABRIC_ROUTER_API_TOKEN precedence, got %q", got)
	}
}

func TestNewCloudflareSyncer_RejectsWhitespaceToken(t *testing.T) {
	_, err := newCloudflareSyncer(cloudflareConfig{
		APIToken: "bad token",
		ZoneID:   "zone-id",
	}, true, nil)
	if err == nil {
		t.Fatal("expected whitespace token to be rejected")
	}
}

func TestResolveCloudflareAPIToken_FromTokenFile(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte("token-from-file\n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN_FILE", tokenFile)
	got := resolveCloudflareAPIToken()
	if got != "token-from-file" {
		t.Fatalf("expected token from file, got %q", got)
	}
}

func TestNewCloudflareSyncer_RejectsPlaceholderToken(t *testing.T) {
	_, err := newCloudflareSyncer(cloudflareConfig{
		APIToken: "replace-me-with-real-token",
		ZoneID:   "zone-id",
	}, true, nil)
	if err == nil {
		t.Fatal("expected placeholder token to be rejected")
	}
}

func TestNewCloudflareSyncer_RejectsTooShortToken(t *testing.T) {
	_, err := newCloudflareSyncer(cloudflareConfig{
		APIToken: "short-token",
		ZoneID:   "zone-id",
	}, true, nil)
	if err == nil {
		t.Fatal("expected too short token to be rejected")
	}
}
