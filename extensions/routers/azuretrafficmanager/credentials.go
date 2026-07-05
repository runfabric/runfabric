package azuretrafficmanager

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the env vars this router reads. The token falls
// back declaratively to AZURE_ACCESS_TOKEN — the same key the
// azure-functions provider uses — so one Azure credential serves deploy and
// traffic-manager sync unless a router-specific token is configured.
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "RUNFABRIC_ROUTER_API_TOKEN", Fallback: "AZURE_ACCESS_TOKEN"},
	{EnvKey: "AZURE_TRAFFIC_MANAGER_PROFILE_ID"},
	{EnvKey: "AZURE_API_BASE_URL", Placeholder: "https://management.azure.com"},
}
