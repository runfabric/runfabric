package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

func TestJournalCheckpointResumeState(t *testing.T) {
	tmp := t.TempDir()
	backend := transactions.NewFileBackend(tmp)

	j := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	_ = j.Checkpoint("package_artifacts", "done")
	_ = j.Checkpoint("discover_state", "done")
	_ = j.Checkpoint("ensure_http_api", "in_progress")

	loaded, err := backend.Load("svc", "dev")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded.Checkpoints) != 3 {
		t.Fatalf("expected 3 checkpoints, got %d", len(loaded.Checkpoints))
	}
}
