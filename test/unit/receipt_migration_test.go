package unit

import (
	"testing"

	"github.com/runfabric/runfabric/internal/state"
)

func TestReceiptMigration(t *testing.T) {
	r := &state.Receipt{
		Version: 0,
		Service: "svc",
		Stage:   "dev",
	}

	out, err := state.MigrateReceipt(r)
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	if out.Version != state.CurrentReceiptVersion {
		t.Fatalf("expected version=%d got=%d", state.CurrentReceiptVersion, out.Version)
	}
}
