package netlify

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// netlifyAPIBase returns the Netlify REST API base URL. When
// NETLIFY_ENDPOINT_URL is set it overrides the real host (for a mock, proxy, or
// emulator), resolved at call time so production is untouched when unset;
// otherwise it falls back to netlifyAPI (the real host).
func netlifyAPIBase() string {
	if b := strings.TrimSpace(sdkprovider.Env("NETLIFY_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return strings.TrimRight(netlifyAPI, "/")
}

// netlifySiteURL returns the public invoke URL for a deployed site. Real Netlify
// serves each site at https://<name>.netlify.app (and returns the exact deploy
// URL in the API response, which Deploy prefers when available); when
// NETLIFY_ENDPOINT_URL is set the site is fronted under the override at
// <base>/app/<name> so a mock or emulator can intercept invocations.
func netlifySiteURL(name string) string {
	if b := strings.TrimSpace(sdkprovider.Env("NETLIFY_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/app/" + name
	}
	return "https://" + name + ".netlify.app"
}
