package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func anyAPIProvider(t *testing.T) string {
	t.Helper()
	names := APIProviderNames()
	if len(names) == 0 {
		t.Skip("no API-dispatch providers registered")
	}
	return names[0]
}

// TestRemove_NoReceiptReportsRemoved verifies that a missing receipt (nothing
// deployed) is reported as already removed.
func TestRemove_NoReceiptReportsRemoved(t *testing.T) {
	root := t.TempDir()
	res, err := Remove(context.Background(), anyAPIProvider(t), &config.Config{}, "dev", root)
	if err != nil {
		t.Fatalf("Remove with no receipt should succeed, got %v", err)
	}
	if res == nil || !res.Removed {
		t.Fatalf("expected Removed=true result, got %+v", res)
	}
}

// TestRemove_CorruptReceiptReturnsError guards the regression where any
// LoadReceipt error (not just not-found) was swallowed as "already removed",
// silently orphaning live cloud resources.
func TestRemove_CorruptReceiptReturnsError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".runfabric")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dev.json"), []byte("{ this is not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt receipt: %v", err)
	}

	res, err := Remove(context.Background(), anyAPIProvider(t), &config.Config{}, "dev", root)
	if err == nil {
		t.Fatal("expected an error for a corrupt receipt, not a false 'removed'")
	}
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
}
