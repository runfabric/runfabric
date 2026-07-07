package gcp

import (
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// gcpHost rewrites the scheme+host of a canonical googleapis.com URL to a local
// emulator base when GCP_ENDPOINT_URL is set (e.g. floci-gcp serving every GCP
// service at http://localhost:4588), preserving the path so the same REST calls
// hit the emulator instead of the real cloud. Without the override it returns
// the canonical URL unchanged, so production behaviour is untouched.
//
// The GCP provider talks to several googleapis.com hosts (cloudfunctions,
// storage, logging, workflowexecutions, cloudtrace); floci-gcp fronts them all
// on one endpoint, so a single override rewrites every host consistently.
func gcpHost(defaultURL string) string {
	base := strings.TrimSpace(sdkprovider.Env("GCP_ENDPOINT_URL"))
	if base == "" {
		return defaultURL
	}
	base = strings.TrimRight(base, "/")
	rest := defaultURL
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if slash := strings.Index(rest, "/"); slash >= 0 {
		return base + rest[slash:]
	}
	return base
}
