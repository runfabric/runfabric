package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func TestPersistentJournal(t *testing.T) {
	tmp := t.TempDir()

	backend := transactions.NewFileBackend(tmp)
	j := transactions.NewJournal("svc", "dev", "deploy", backend)

	if err := j.Save(); err != nil {
		t.Fatalf("save journal failed: %v", err)
	}

	if err := j.Record(transactions.Operation{
		Type:     transactions.OpCreateLambda,
		Resource: "lambda-1",
		Metadata: map[string]string{"functionName": "lambda-1"},
	}); err != nil {
		t.Fatalf("record journal op failed: %v", err)
	}

	if err := j.MarkCompleted(); err != nil {
		t.Fatalf("mark completed failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load journal failed: %v", err)
	}

	if loaded.Status != transactions.StatusCompleted {
		t.Fatalf("expected completed status, got %s", loaded.Status)
	}

	if len(loaded.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(loaded.Operations))
	}
}
