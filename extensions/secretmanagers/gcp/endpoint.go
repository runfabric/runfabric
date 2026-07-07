package main

import "strings"

// secretManagerHost rewrites the scheme+host of a real
// secretmanager.googleapis.com URL to a local emulator/proxy base when
// GCP_ENDPOINT_URL is set (e.g. http://localhost:4588), preserving the path and
// query so the same REST call hits the override instead of the real cloud.
// When override is empty it returns the input URL unchanged, so production
// behaviour is untouched.
//
// floci-gcp does not emulate Secret Manager, so this override is exercised by a
// unit-test HTTP double rather than a live emulator.
func secretManagerHost(defaultURL, override string) string {
	base := strings.TrimSpace(override)
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
