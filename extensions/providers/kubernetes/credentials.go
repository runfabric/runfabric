package kubernetes

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads. All are
// env-only (no Header): kubeconfig is file-based and registry credentials feed
// a local image build, so neither can be supplied per daemon request.
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "KUBECONFIG", Required: true, Placeholder: "~/.kube/config"},
	{EnvKey: "GHCR_REGISTRY"},
	{EnvKey: "GHCR_USER"},
	{EnvKey: "GHCR_TOKEN"},
}
