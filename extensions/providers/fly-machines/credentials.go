package fly

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads
// (Fly deploys a prebuilt image, so FLY_IMAGE is a hard requirement).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "FLY_API_TOKEN", Header: "X-Provider-Fly-Api-Token", Required: true},
	{EnvKey: "FLY_IMAGE", Header: "X-Provider-Fly-Image", Required: true, Placeholder: "registry.fly.io/myapp:latest"},
	{EnvKey: "FLY_ORG_ID", Header: "X-Provider-Fly-Org-Id"},
}
