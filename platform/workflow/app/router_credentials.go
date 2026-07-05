package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// routerAPITokenEnv is the env var every router plugin reads its API token from.
const routerAPITokenEnv = "RUNFABRIC_ROUTER_API_TOKEN"

// PrimeRouterAPIToken makes the router API token available to the router
// plugin for the duration of a sync by exporting it as RUNFABRIC_ROUTER_API_TOKEN.
//
// Resolution precedence (mirrors the router CLI):
//  1. RUNFABRIC_ROUTER_API_TOKEN already set — leave untouched.
//  2. The policy's custom token env (extensions.router.credentials.apiTokenEnv).
//  3. The policy's secret reference (credentials.apiTokenSecretRef), resolved
//     through the RunFabric secret subsystem — top-level `secrets:` map,
//     ${secret:KEY} / secret://KEY indirection, and secret-manager plugin refs
//     (aws-sm://, vault://, ...) when extensions.secretManagerPlugin is
//     configured. Bootstrap installs that resolver before deploy runs, so this
//     works on both the CLI and the daemon (/deploy) paths.
//  4. A token file named by credentials.apiTokenFileEnv.
//
// The returned restore func reinstates the previous environment. The daemon is
// a long-running process serving many projects, so a token resolved for one
// sync must not linger in the process env after the sync completes.
func PrimeRouterAPIToken(cfg *config.Config, policy RouterDNSSyncPolicy) (func(), error) {
	noop := func() {}
	if strings.TrimSpace(os.Getenv(routerAPITokenEnv)) != "" {
		return noop, nil
	}
	token, err := resolveRouterAPIToken(cfg, policy)
	if err != nil {
		return noop, err
	}
	if token == "" {
		// No token source configured — plugins fall back to their native
		// credential chains (e.g. route53 uses the AWS default chain).
		return noop, nil
	}
	if err := os.Setenv(routerAPITokenEnv, token); err != nil {
		return noop, err
	}
	return func() { _ = os.Unsetenv(routerAPITokenEnv) }, nil
}

// RouterProviderIDs reads the DNS zone and account IDs from the env vars named
// by the credential policy (defaults RUNFABRIC_ROUTER_ZONE_ID /
// RUNFABRIC_ROUTER_ACCOUNT_ID), for sync paths with no CLI flags to supply them.
func RouterProviderIDs(policy RouterDNSSyncPolicy) (zoneID, accountID string) {
	zoneEnv := strings.TrimSpace(policy.ZoneIDEnv)
	if zoneEnv == "" {
		zoneEnv = "RUNFABRIC_ROUTER_ZONE_ID"
	}
	accountEnv := strings.TrimSpace(policy.AccountIDEnv)
	if accountEnv == "" {
		accountEnv = "RUNFABRIC_ROUTER_ACCOUNT_ID"
	}
	return strings.TrimSpace(os.Getenv(zoneEnv)), strings.TrimSpace(os.Getenv(accountEnv))
}

// resolveRouterAPIToken finds a token via policy env → secret ref → token file.
func resolveRouterAPIToken(cfg *config.Config, policy RouterDNSSyncPolicy) (string, error) {
	if env := strings.TrimSpace(policy.APITokenEnv); env != "" && env != routerAPITokenEnv {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, nil
		}
	}
	if ref := strings.TrimSpace(policy.APITokenSecretRef); ref != "" {
		token, err := ResolveRouterAPITokenSecretRef(cfg, ref)
		if err != nil {
			return "", fmt.Errorf("resolve router API token secret ref: %w", err)
		}
		return token, nil
	}
	fileEnv := strings.TrimSpace(policy.APITokenFileEnv)
	if fileEnv == "" {
		fileEnv = "RUNFABRIC_ROUTER_API_TOKEN_FILE"
	}
	if path := strings.TrimSpace(os.Getenv(fileEnv)); path != "" {
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return "", fmt.Errorf("read router API token file %q: %w", path, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("router API token file %q is empty", path)
		}
		return token, nil
	}
	return "", nil
}
