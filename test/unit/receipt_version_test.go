package unit

import (
	"testing"

	"github.com/runfabric/runfabric/internal/state"
)

func TestReceiptCurrentVersion(t *testing.T) {
	r := &state.Receipt{
		Service: "svc",
		Stage:   "dev",
	}

	tmp := t.TempDir()
	if err := state.Save(tmp, r); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := state.Load(tmp, "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Version != state.CurrentReceiptVersion {
		t.Fatalf("expected receipt version %d got %d", state.CurrentReceiptVersion, loaded.Version)
	}
}
