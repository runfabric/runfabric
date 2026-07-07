package azure

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// azureManagementBase returns the ARM management endpoint. When AZURE_ENDPOINT_URL
// is set (e.g. floci-az fronting ARM on http://localhost:4577) it overrides the
// real management host so the provider's control-plane REST calls hit the
// emulator; otherwise it falls back to azureManagementAPI (the real host, or a
// value a test swaps in). Production behaviour is unchanged when the override is
// empty.
func azureManagementBase() string {
	if b := strings.TrimSpace(sdkprovider.Env("AZURE_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return strings.TrimRight(azureManagementAPI, "/")
}

// azureAppHost returns the data-plane base URL for a function app's HTTP
// endpoints (used to build per-function invoke URLs). Real Azure serves each app
// at https://<app>.azurewebsites.net; when AZURE_ENDPOINT_URL is set the app is
// fronted under the override at <base>/app/<app> so an emulator or test double
// can intercept invocations.
func azureAppHost(appName string) string {
	if b := strings.TrimSpace(sdkprovider.Env("AZURE_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/app/" + appName
	}
	return "https://" + appName + ".azurewebsites.net"
}

// azureScmHost returns the Kudu/SCM base URL used for zip deployment. Real Azure
// serves it at https://<app>.scm.azurewebsites.net; under AZURE_ENDPOINT_URL it
// is fronted at <base>/scm/<app>.
func azureScmHost(appName string) string {
	if b := strings.TrimSpace(sdkprovider.Env("AZURE_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/scm/" + appName
	}
	return "https://" + appName + ".scm.azurewebsites.net"
}
