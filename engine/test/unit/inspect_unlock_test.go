package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/locking"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

func TestInspectableArtifactsExist(t *testing.T) {
	tmp := t.TempDir()

	lockBackend := locking.NewFileBackend(tmp)
	_, err := lockBackend.Acquire("svc", "dev", "deploy", 0)
	if err != nil {
		t.Fatalf("acquire lock failed: %v", err)
	}

	journalBackend := transactions.NewFileBackend(tmp)
	journal := transactions.NewJournal("svc", "dev", "deploy", journalBackend)
	if err := journal.Save(); err != nil {
		t.Fatalf("save journal failed: %v", err)
	}

	lockPath := filepath.Join(tmp, ".runfabric", "locks", "svc-dev.lock.json")
	journalPath := filepath.Join(tmp, ".runfabric", "journals", "svc-dev.journal.json")

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file: %v", err)
	}
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("expected journal file: %v", err)
	}
}
