package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMigrateReceipt_AcceptsLegacyVersions guards the regression where any
// version other than the current one was rejected, making pre-versioning (0) and
// v1 receipts unloadable even though the schema is additive.
func TestMigrateReceipt_AcceptsLegacyVersions(t *testing.T) {
	for _, v := range []int{0, 1, CurrentReceiptVersion} {
		out, err := MigrateReceipt(&Receipt{Version: v, Service: "svc", Stage: "dev"})
		if err != nil {
			t.Fatalf("version %d: unexpected error %v", v, err)
		}
		if out.Version != CurrentReceiptVersion {
			t.Errorf("version %d: migrated to %d, want %d", v, out.Version, CurrentReceiptVersion)
		}
	}
}

// TestMigrateReceipt_RejectsNewerVersion verifies a receipt from a newer binary
// is rejected rather than silently downgraded.
func TestMigrateReceipt_RejectsNewerVersion(t *testing.T) {
	if _, err := MigrateReceipt(&Receipt{Version: CurrentReceiptVersion + 1}); err == nil {
		t.Fatal("expected error for a newer-than-supported receipt version")
	}
}

// TestLoad_LegacyReceiptRoundTrips verifies a legacy (version 1) receipt on disk
// loads successfully instead of erroring out the deploy/remove/releases commands.
func TestLoad_LegacyReceiptRoundTrips(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A v1 receipt with only the fields that existed then.
	legacy := `{"version":1,"service":"svc","stage":"dev","provider":"aws-lambda","outputs":{"url":"https://x"},"updatedAt":"2024-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "dev.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy receipt: %v", err)
	}

	r, err := Load(root, "dev")
	if err != nil {
		t.Fatalf("Load legacy receipt: %v", err)
	}
	if r.Service != "svc" || r.Provider != "aws-lambda" || r.Outputs["url"] != "https://x" {
		t.Errorf("legacy fields not preserved: %+v", r)
	}
	if r.Version != CurrentReceiptVersion {
		t.Errorf("loaded version = %d, want migrated to %d", r.Version, CurrentReceiptVersion)
	}
}
