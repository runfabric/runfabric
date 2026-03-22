package unit

import (
	"testing"

	state "github.com/runfabric/runfabric/platform/core/state/core"
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

func TestReceiptMigration_Version1(t *testing.T) {
	r := &state.Receipt{
		Version: 1,
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
