package cloudflare

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads.
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "CLOUDFLARE_ACCOUNT_ID", Header: "X-Provider-Cloudflare-Account-Id", Required: true},
	{EnvKey: "CLOUDFLARE_API_TOKEN", Header: "X-Provider-Cloudflare-Api-Token", Required: true},
}
