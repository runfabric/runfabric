package ns1

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the env vars this router reads. The token falls
// back declaratively to the native NS1 key so existing NS1 setups work
// without exporting the router-specific var.
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "RUNFABRIC_ROUTER_API_TOKEN", Fallback: "NS1_API_KEY"},
	{EnvKey: "NS1_API_BASE_URL", Placeholder: "https://api.nsone.net"},
}
