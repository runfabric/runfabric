package azure

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads
// (AZURE_RESOURCE_GROUP defaults to <service>-<stage> when unset).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "AZURE_ACCESS_TOKEN", Header: "X-Provider-Azure-Access-Token", Required: true},
	{EnvKey: "AZURE_SUBSCRIPTION_ID", Header: "X-Provider-Azure-Subscription-Id", Required: true},
	{EnvKey: "AZURE_RESOURCE_GROUP", Header: "X-Provider-Azure-Resource-Group"},
}
