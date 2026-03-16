package unit

import (
	"os"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/recovery"
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

func TestArchiveJournal(t *testing.T) {
	tmp := t.TempDir()

	jf := &transactions.JournalFile{
		Service:   "svc",
		Stage:     "dev",
		Operation: "deploy",
		Status:    transactions.StatusActive,
	}

	path, err := recovery.ArchiveJournal(tmp, jf)
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected archive file: %v", err)
	}
}
