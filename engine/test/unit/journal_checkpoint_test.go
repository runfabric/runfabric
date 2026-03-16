package unit

import (
	"testing"

	"github.com/runfabric/runfabric/engine/internal/transactions"
)

func TestJournalCheckpoints(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := j.Checkpoint("package_artifacts", "done"); err != nil {
		t.Fatalf("checkpoint failed: %v", err)
	}
	if err := j.Checkpoint("ensure_api", "done"); err != nil {
		t.Fatalf("checkpoint failed: %v", err)
	}
	if err := j.Checkpoint("ensure_routes", "in_progress"); err != nil {
		t.Fatalf("checkpoint failed: %v", err)
	}

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded.Checkpoints) != 3 {
		t.Fatalf("expected 3 checkpoints, got %d", len(loaded.Checkpoints))
	}
}
