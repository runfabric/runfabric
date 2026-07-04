package digitalocean

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads
// (deploys go through App Platform, so the GitHub repo is a hard requirement).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "DIGITALOCEAN_ACCESS_TOKEN", Header: "X-Provider-Digitalocean-Access-Token", Required: true},
	{EnvKey: "DO_APP_REPO", Header: "X-Provider-Digitalocean-App-Repo", Required: true, Placeholder: "owner/repo"},
	{EnvKey: "DO_REGION", Header: "X-Provider-Digitalocean-Region", Placeholder: "nyc"},
}
