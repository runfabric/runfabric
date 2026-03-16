package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPrepareDevStreamTunnel_NonAWS_ReturnsNilNil ensures that for providers other than AWS
// (e.g. GCP, Cloudflare), PrepareDevStreamTunnel returns (nil, nil) so dev still runs the local server without auto-wire.
func TestPrepareDevStreamTunnel_NonAWS_ReturnsNilNil(t *testing.T) {
	dir := t.TempDir()
	cfg := `service: test-dev-stream
provider:
  name: gcp-functions
  runtime: nodejs
  region: us-central1
functions:
  api:
    handler: index.handler
`
	cfgPath := filepath.Join(dir, "runfabric.yml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	restore, err := PrepareDevStreamTunnel(cfgPath, "dev", "https://abc.ngrok.io")
	if err != nil {
		t.Fatalf("expected no error for GCP provider, got %v", err)
	}
	if restore != nil {
		t.Fatal("expected nil restore for GCP provider (no auto-wire)")
	}
}
