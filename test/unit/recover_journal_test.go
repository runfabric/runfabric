package unit

import (
	"testing"

	"github.com/runfabric/runfabric/internal/transactions"
)

func TestJournalLifecycle(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := j.Record(transactions.Operation{
		Type:     transactions.OpCreateLambda,
		Resource: "lambda-1",
	}); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	if err := j.MarkRollingBack(); err != nil {
		t.Fatalf("mark rolling_back failed: %v", err)
	}

	if err := j.MarkRolledBack(); err != nil {
		t.Fatalf("mark rolled_back failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.Status != transactions.StatusRolledBack {
		t.Fatalf("expected rolled_back, got %s", loaded.Status)
	}
}
