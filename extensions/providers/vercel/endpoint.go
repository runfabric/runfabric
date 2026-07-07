package vercel

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// vercelAPIBase returns the Vercel REST API base URL. When VERCEL_ENDPOINT_URL is
// set it overrides the real host (for a mock, proxy, or emulator), resolved at
// call time so production is untouched when unset; otherwise it falls back to
// vercelAPI (the real host).
func vercelAPIBase() string {
	if b := strings.TrimSpace(sdkprovider.Env("VERCEL_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return strings.TrimRight(vercelAPI, "/")
}

// vercelDeploymentURL returns the invoke URL for a deployed project. Real Vercel
// serves it at https://<name>.vercel.app; under VERCEL_ENDPOINT_URL the
// deployment is fronted at <base>/app/<name> so a mock or emulator can serve
// invocations.
func vercelDeploymentURL(name string) string {
	if b := strings.TrimSpace(sdkprovider.Env("VERCEL_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/app/" + name
	}
	return "https://" + name + ".vercel.app"
}
