package digitalocean

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// digitaloceanAPIBase returns the DigitalOcean App Platform apps-collection
// endpoint. When DIGITALOCEAN_ENDPOINT_URL is set it overrides the real host
// (for a mock, proxy, or emulator), resolved at call time so production is
// untouched when unset; otherwise it falls back to doAPI (the real host).
func digitaloceanAPIBase() string {
	if b := strings.TrimSpace(sdkprovider.Env("DIGITALOCEAN_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/v2/apps"
	}
	return strings.TrimRight(doAPI, "/")
}

// digitaloceanAppURL returns the invoke URL for a deployed app. Real
// DigitalOcean returns the live URL in the create-app response; under
// DIGITALOCEAN_ENDPOINT_URL the app is fronted at <base>/app/<name> so a mock or
// emulator can serve invocations. liveURL is the value from the API response,
// used verbatim when the override is empty.
func digitaloceanAppURL(name, liveURL string) string {
	if b := strings.TrimSpace(sdkprovider.Env("DIGITALOCEAN_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/app/" + name
	}
	return liveURL
}
