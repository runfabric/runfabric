package cloudflare

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the env vars this router reads, with declarative
// same-cloud fallbacks to the cloudflare-workers provider keys — one set of
// provider credentials serves deploy AND DNS sync unless router-specific
// values are configured (which always win, incl. daemon X-Router-* headers).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "RUNFABRIC_ROUTER_API_TOKEN", Fallback: "CLOUDFLARE_API_TOKEN"},
	{EnvKey: "RUNFABRIC_ROUTER_API_TOKEN_FILE", Placeholder: "/run/secrets/cf-token"},
	{EnvKey: "RUNFABRIC_ROUTER_ZONE_ID", Fallback: "CLOUDFLARE_ZONE_ID"},
	{EnvKey: "RUNFABRIC_ROUTER_ACCOUNT_ID", Fallback: "CLOUDFLARE_ACCOUNT_ID"},
}
