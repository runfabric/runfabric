package cloudflare

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// cloudflareAPIBase returns the Cloudflare API base URL. When
// CLOUDFLARE_ENDPOINT_URL is set it overrides the real host (for a mock, proxy,
// or emulator), resolved at call time so production is untouched when unset;
// otherwise it falls back to cfAPI (the real host, or a value a test swaps in).
func cloudflareAPIBase() string {
	if b := strings.TrimSpace(sdkprovider.Env("CLOUDFLARE_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return strings.TrimRight(cfAPI, "/")
}

// cloudflareWorkerURL returns the invoke URL for a deployed worker. Real
// Cloudflare serves it at https://<name>.workers.dev; under
// CLOUDFLARE_ENDPOINT_URL the worker is fronted at <base>/app/<name> so a mock or
// emulator can serve invocations.
func cloudflareWorkerURL(name string) string {
	if b := strings.TrimSpace(sdkprovider.Env("CLOUDFLARE_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/app/" + name
	}
	return "https://" + name + ".workers.dev"
}
