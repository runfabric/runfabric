package netlify

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads
// (a site is created automatically when NETLIFY_SITE_ID is unset).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "NETLIFY_AUTH_TOKEN", Header: "X-Provider-Netlify-Auth-Token", Required: true},
	{EnvKey: "NETLIFY_SITE_ID", Header: "X-Provider-Netlify-Site-Id"},
}
