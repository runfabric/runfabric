package fly

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// flyAPIBase returns the Fly Machines API base URL. When FLY_ENDPOINT_URL is set
// it overrides the real host (for a mock, proxy, or emulator), resolved at call
// time so production is untouched when unset; otherwise it falls back to flyAPI
// (the real host).
func flyAPIBase() string {
	if b := strings.TrimSpace(sdkprovider.Env("FLY_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return strings.TrimRight(flyAPI, "/")
}

// flyAppURL returns the invoke URL for a deployed app. Real Fly serves it at
// https://<app>.fly.dev; under FLY_ENDPOINT_URL the app is fronted at
// <base>/app/<app> so a mock or emulator can serve invocations.
func flyAppURL(appName string) string {
	if b := strings.TrimSpace(sdkprovider.Env("FLY_ENDPOINT_URL")); b != "" {
		return strings.TrimRight(b, "/") + "/app/" + appName
	}
	return "https://" + appName + ".fly.dev"
}
