package vercel

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads.
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "VERCEL_TOKEN", Header: "X-Provider-Vercel-Token", Required: true},
	{EnvKey: "VERCEL_TEAM_ID", Header: "X-Provider-Vercel-Team-Id"},
}
