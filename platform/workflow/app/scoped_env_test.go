package app

import (
	"os"
	"testing"
)

func clearVars(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestProviderKeysAreNotPromoted(t *testing.T) {
	// Provider credentials stay native-only: no RUNFABRIC_PROVIDER_* spelling.
	clearVars(t, "AWS_ACCESS_KEY_ID", "CLOUDFLARE_API_TOKEN")
	t.Setenv("RUNFABRIC_PROVIDER_AWS_ACCESS_KEY_ID", "AKIA_SCOPED")
	t.Setenv("RUNFABRIC_PROVIDER_CLOUDFLARE_API_TOKEN", "cf-scoped")

	promoteScopedCredentialEnv()

	if _, set := os.LookupEnv("AWS_ACCESS_KEY_ID"); set {
		t.Fatal("provider keys must not be promoted from RUNFABRIC_PROVIDER_*")
	}
	if _, set := os.LookupEnv("CLOUDFLARE_API_TOKEN"); set {
		t.Fatal("provider keys must not be promoted from RUNFABRIC_PROVIDER_*")
	}
}

func TestNativeWinsOverScoped(t *testing.T) {
	// e.g. set by a daemon X-Secret-Vault-Token header for this request.
	t.Setenv("VAULT_TOKEN", "hvs.native")
	t.Setenv("RUNFABRIC_SM_VAULT_TOKEN", "hvs.scoped")

	promoteScopedCredentialEnv()

	if got := os.Getenv("VAULT_TOKEN"); got != "hvs.native" {
		t.Fatalf("VAULT_TOKEN = %q — native/per-request value must win", got)
	}
}

func TestScopedFormsAcrossKinds(t *testing.T) {
	clearVars(t, "VAULT_TOKEN", "AZURE_STORAGE_KEY", "AZURE_API_BASE_URL")
	t.Setenv("RUNFABRIC_SM_VAULT_TOKEN", "hvs.scoped")
	t.Setenv("RUNFABRIC_STATE_AZURE_STORAGE_KEY", "az-scoped")
	// Router plugin var (azuretrafficmanager declares AZURE_API_BASE_URL).
	t.Setenv("RUNFABRIC_ROUTER_AZURE_API_BASE_URL", "https://mgmt.example")

	promoteScopedCredentialEnv()

	if got := os.Getenv("VAULT_TOKEN"); got != "hvs.scoped" {
		t.Fatalf("VAULT_TOKEN = %q", got)
	}
	if got := os.Getenv("AZURE_STORAGE_KEY"); got != "az-scoped" {
		t.Fatalf("AZURE_STORAGE_KEY = %q", got)
	}
	if got := os.Getenv("AZURE_API_BASE_URL"); got != "https://mgmt.example" {
		t.Fatalf("AZURE_API_BASE_URL = %q", got)
	}
}

func TestAlreadyPrefixedKeysGetNoSecondSpelling(t *testing.T) {
	// RUNFABRIC_ROUTER_API_TOKEN is already the native name — the promotion
	// must not look for RUNFABRIC_ROUTER_RUNFABRIC_ROUTER_API_TOKEN.
	clearVars(t, "RUNFABRIC_ROUTER_API_TOKEN")
	t.Setenv("RUNFABRIC_ROUTER_RUNFABRIC_ROUTER_API_TOKEN", "nonsense")

	promoteScopedCredentialEnv()

	if _, set := os.LookupEnv("RUNFABRIC_ROUTER_API_TOKEN"); set {
		t.Fatal("already-namespaced native keys must not be promoted into")
	}
}
