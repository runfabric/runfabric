package unit

import (
	"testing"

	"github.com/runfabric/runfabric/internal/errors"
	"github.com/runfabric/runfabric/internal/transactions"
)

func TestJournalConflictErrorFields(t *testing.T) {
	err := &errors.ConflictError{
		Backend:         "s3",
		Service:         "billing",
		Stage:           "dev",
		Resource:        "journal",
		CurrentVersion:  7,
		IncomingVersion: 6,
		Action:          "inspect journal and retry with latest state",
	}

	if err.Backend != "s3" {
		t.Fatalf("expected backend=s3, got %q", err.Backend)
	}
	if err.Service != "billing" {
		t.Fatalf("expected service=billing, got %q", err.Service)
	}
	if err.Stage != "dev" {
		t.Fatalf("expected stage=dev, got %q", err.Stage)
	}
	if err.Resource != "journal" {
		t.Fatalf("expected resource=journal, got %q", err.Resource)
	}
	if err.CurrentVersion != 7 {
		t.Fatalf("expected current version=7, got %d", err.CurrentVersion)
	}
	if err.IncomingVersion != 6 {
		t.Fatalf("expected incoming version=6, got %d", err.IncomingVersion)
	}
	if err.Action == "" {
		t.Fatal("expected non-empty action")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error string")
	}
}

func TestJournalConflictDoesNotOverwriteCurrentState(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j1 := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j1.Save(); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	if err := j1.Record(transactions.Operation{
		Type:     transactions.OpCreateLambda,
		Resource: "lambda-1",
	}); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	current, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load current journal failed: %v", err)
	}

	// Simulate a stale writer with an older version than current.
	stale := *current
	stale.Version = current.Version - 1

	saveErr := backend.Save(&stale)
	if saveErr == nil {
		t.Fatal("expected stale save to fail with conflict")
	}

	reloaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if reloaded.Version != current.Version {
		t.Fatalf("expected version to remain %d, got %d", current.Version, reloaded.Version)
	}
	if len(reloaded.Operations) != len(current.Operations) {
		t.Fatalf("expected operations length to remain %d, got %d", len(current.Operations), len(reloaded.Operations))
	}
}

func TestJournalSaveRejectsStaleVersion(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	current, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	stale := *current
	stale.Version = current.Version - 1

	if err := backend.Save(&stale); err == nil {
		t.Fatal("expected backend.Save to reject stale version")
	}
}
