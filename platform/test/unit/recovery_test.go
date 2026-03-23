package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func TestRecoveryDetectsUnfinishedJournal(t *testing.T) {
	tmp := t.TempDir()

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("save journal failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load journal failed: %v", err)
	}

	if loaded.Status != transactions.StatusActive {
		t.Fatalf("expected active status, got %s", loaded.Status)
	}
}
