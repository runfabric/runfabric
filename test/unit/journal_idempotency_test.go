package unit

import (
	"testing"

	"github.com/runfabric/runfabric/internal/deployexec"
	"github.com/runfabric/runfabric/internal/transactions"
)

func TestRecordOnce(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	op := transactions.Operation{
		Type:     transactions.OpCreateLambda,
		Resource: "lambda-hello",
	}

	if err := deployexec.RecordOnce(j, op); err != nil {
		t.Fatalf("record once failed: %v", err)
	}
	if err := deployexec.RecordOnce(j, op); err != nil {
		t.Fatalf("second record once failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(loaded.Operations))
	}
}
