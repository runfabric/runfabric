package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestPrimeRouterAPITokenKeepsExistingEnv(t *testing.T) {
	t.Setenv(routerAPITokenEnv, "already-set")
	restore, err := PrimeRouterAPIToken(nil, RouterDNSSyncPolicy{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	restore()
	if got := os.Getenv(routerAPITokenEnv); got != "already-set" {
		t.Fatalf("existing token must be untouched, got %q", got)
	}
}

func TestPrimeRouterAPITokenFromPolicyEnv(t *testing.T) {
	t.Setenv(routerAPITokenEnv, "")
	_ = os.Unsetenv(routerAPITokenEnv)
	t.Setenv("CUSTOM_ROUTER_TOKEN", "from-custom-env")

	restore, err := PrimeRouterAPIToken(nil, RouterDNSSyncPolicy{APITokenEnv: "CUSTOM_ROUTER_TOKEN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv(routerAPITokenEnv); got != "from-custom-env" {
		t.Fatalf("token not primed from policy env, got %q", got)
	}
	restore()
	if _, set := os.LookupEnv(routerAPITokenEnv); set {
		t.Fatal("restore must unset the primed token")
	}
}

func TestPrimeRouterAPITokenFromSecretRef(t *testing.T) {
	t.Setenv(routerAPITokenEnv, "")
	_ = os.Unsetenv(routerAPITokenEnv)

	cfg := &config.Config{Secrets: map[string]string{"ROUTER_TOKEN": "from-secrets-map"}}
	restore, err := PrimeRouterAPIToken(cfg, RouterDNSSyncPolicy{APITokenSecretRef: "secret://ROUTER_TOKEN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv(routerAPITokenEnv); got != "from-secrets-map" {
		t.Fatalf("token not primed from secret ref, got %q", got)
	}
	restore()
	if _, set := os.LookupEnv(routerAPITokenEnv); set {
		t.Fatal("restore must unset the primed token")
	}
}

func TestPrimeRouterAPITokenSecretRefError(t *testing.T) {
	t.Setenv(routerAPITokenEnv, "")
	_ = os.Unsetenv(routerAPITokenEnv)

	_, err := PrimeRouterAPIToken(&config.Config{}, RouterDNSSyncPolicy{APITokenSecretRef: "secret://MISSING_KEY"})
	if err == nil {
		t.Fatal("expected an error for an unresolvable secret ref")
	}
	if _, set := os.LookupEnv(routerAPITokenEnv); set {
		t.Fatal("a failed resolution must not leave a token in the env")
	}
}

func TestPrimeRouterAPITokenFromFile(t *testing.T) {
	t.Setenv(routerAPITokenEnv, "")
	_ = os.Unsetenv(routerAPITokenEnv)

	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN_FILE", path)

	restore, err := PrimeRouterAPIToken(nil, RouterDNSSyncPolicy{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv(routerAPITokenEnv); got != "from-file" {
		t.Fatalf("token not primed (trimmed) from file, got %q", got)
	}
	restore()
}

func TestPrimeRouterAPITokenNoSourceIsNoop(t *testing.T) {
	t.Setenv(routerAPITokenEnv, "")
	_ = os.Unsetenv(routerAPITokenEnv)
	t.Setenv("RUNFABRIC_ROUTER_API_TOKEN_FILE", "")

	restore, err := PrimeRouterAPIToken(nil, RouterDNSSyncPolicy{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	restore()
	if _, set := os.LookupEnv(routerAPITokenEnv); set {
		t.Fatal("no source configured must leave the env unset")
	}
}

func TestRouterProviderIDs(t *testing.T) {
	t.Setenv("RUNFABRIC_ROUTER_ZONE_ID", "zone-default")
	t.Setenv("RUNFABRIC_ROUTER_ACCOUNT_ID", "account-default")
	zone, account := RouterProviderIDs(RouterDNSSyncPolicy{})
	if zone != "zone-default" || account != "account-default" {
		t.Fatalf("default env keys not read: zone=%q account=%q", zone, account)
	}

	t.Setenv("MY_ZONE", "zone-custom")
	t.Setenv("MY_ACCOUNT", "account-custom")
	zone, account = RouterProviderIDs(RouterDNSSyncPolicy{ZoneIDEnv: "MY_ZONE", AccountIDEnv: "MY_ACCOUNT"})
	if zone != "zone-custom" || account != "account-custom" {
		t.Fatalf("policy env keys not read: zone=%q account=%q", zone, account)
	}
}
