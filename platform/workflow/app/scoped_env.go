package app

import (
	"os"
	"strings"

	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

// Scoped credential env convention for the NON-provider extension kinds.
// Provider credentials deliberately stay on their native names (AWS_*,
// CLOUDFLARE_API_TOKEN, ...) — the provider identity is the base identity,
// and dependent extensions fall back to it via declarative CredentialVar
// Fallback entries (see plugin-sdk provider.ResolveVar). Declared vars of the
// other kinds whose native name is not already RUNFABRIC_-prefixed can also
// be supplied as:
//
//	RUNFABRIC_STATE_<KEY>   → state backend vars   (e.g. RUNFABRIC_STATE_AZURE_STORAGE_KEY)
//	RUNFABRIC_SM_<KEY>      → secret manager vars  (e.g. RUNFABRIC_SM_VAULT_TOKEN)
//	RUNFABRIC_ROUTER_<KEY>  → router plugin vars   (e.g. RUNFABRIC_ROUTER_AZURE_API_BASE_URL)
//
// Bootstrap resolves the scoped form into the native var so backends and
// plugin processes (which inherit the env) keep reading their standard names.
//
// Precedence: the NATIVE var wins when already set. This keeps existing
// setups untouched and, critically, keeps daemon per-request credentials
// authoritative — X-State-*/X-Secret-* headers write the native keys, which
// an ambient scoped var must never override. Scopes resolve in a fixed order
// (state, sm, router); the first scoped var to fill a native key wins.
func promoteScopedCredentialEnv() {
	type scope struct {
		prefix string
		vars   []catalog.CredentialVar
	}
	scopes := []scope{
		{prefix: "RUNFABRIC_STATE_"},
		{prefix: "RUNFABRIC_SM_"},
		{prefix: "RUNFABRIC_ROUTER_"},
	}
	for _, creds := range providerpolicy.AllStateBackendCredentials() {
		scopes[0].vars = append(scopes[0].vars, creds...)
	}
	for _, creds := range providerpolicy.SecretManagerCredentialVars() {
		scopes[1].vars = append(scopes[1].vars, creds...)
	}
	for _, creds := range providerpolicy.RouterPluginCredentialVars() {
		scopes[2].vars = append(scopes[2].vars, creds...)
	}

	for _, s := range scopes {
		for _, c := range s.vars {
			native := strings.TrimSpace(c.EnvKey)
			// Already-namespaced vars (RUNFABRIC_S3_BUCKET, RUNFABRIC_SM_AWS_*,
			// RUNFABRIC_ROUTER_API_TOKEN, ...) need no second spelling.
			if native == "" || strings.HasPrefix(native, "RUNFABRIC_") {
				continue
			}
			if _, set := os.LookupEnv(native); set {
				continue // native (or a per-request header that wrote it) wins
			}
			scopedVal := strings.TrimSpace(os.Getenv(s.prefix + native))
			if scopedVal == "" {
				continue
			}
			_ = os.Setenv(native, scopedVal)
			// Keep declared mirrors in lockstep, exactly like header application
			// does (e.g. AWS_REGION → AWS_DEFAULT_REGION).
			if mirror := strings.TrimSpace(c.Mirror); mirror != "" {
				if _, set := os.LookupEnv(mirror); !set {
					_ = os.Setenv(mirror, scopedVal)
				}
			}
		}
	}
}
