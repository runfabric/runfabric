package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func TestJournalIncrementAttempt(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := j.IncrementAttempt(); err != nil {
		t.Fatalf("increment attempt failed: %v", err)
	}
	if err := j.IncrementAttempt(); err != nil {
		t.Fatalf("second increment attempt failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.AttemptCount != 2 {
		t.Fatalf("expected attemptCount=2, got %d", loaded.AttemptCount)
	}
}
