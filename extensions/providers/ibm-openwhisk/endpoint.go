package ibm

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// openwhiskDefaultAPIHost is the real IBM Cloud Functions (OpenWhisk) API host,
// used when neither the endpoint override nor IBM_OPENWHISK_API_HOST is set.
const openwhiskDefaultAPIHost = "https://us-south.functions.cloud.ibm.com"

// openwhiskAPIBase returns the OpenWhisk API base URL. When
// IBM_OPENWHISK_ENDPOINT_URL is set it overrides the real host (for a mock,
// proxy, or emulator), resolved at call time so production is untouched when
// unset; otherwise it falls back to IBM_OPENWHISK_API_HOST (the real host, or a
// value a test swaps in), and finally the default IBM Cloud Functions host.
// OpenWhisk serves deploy, invoke, remove, and logs from this single API host,
// so every control- and data-plane call routes through this helper.
func openwhiskAPIBase() string {
	host := strings.TrimSpace(sdkprovider.Env("IBM_OPENWHISK_ENDPOINT_URL"))
	if host == "" {
		host = strings.TrimSpace(sdkprovider.Env("IBM_OPENWHISK_API_HOST"))
	}
	if host == "" {
		host = openwhiskDefaultAPIHost
	}
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	return strings.TrimRight(host, "/")
}
