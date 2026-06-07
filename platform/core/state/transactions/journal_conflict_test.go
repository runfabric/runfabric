package transactions

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/errors"
)

// TestFileBackend_Save_EqualVersionConflict guards the lost-update regression:
// a write whose version equals what is already on disk must be rejected, not
// silently overwrite the other writer's revision.
func TestFileBackend_Save_EqualVersionConflict(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)

	j1 := &JournalFile{Service: "svc", Stage: "dev", Operation: "deploy", Status: StatusActive, Version: 2}
	if err := b.Save(j1); err != nil {
		t.Fatalf("first save: %v", err)
	}

	j2 := &JournalFile{Service: "svc", Stage: "dev", Operation: "deploy", Status: StatusActive, Version: 2}
	err := b.Save(j2)
	conflict, ok := err.(*errors.ConflictError)
	if err == nil || !ok {
		t.Fatalf("expected ConflictError for equal version, got %v", err)
	}
	if conflict.CurrentVersion != 2 || conflict.IncomingVersion != 2 {
		t.Errorf("ConflictError: current=%d incoming=%d", conflict.CurrentVersion, conflict.IncomingVersion)
	}
}

// TestJournal_ConcurrentWritersLostUpdateDetected verifies that two journals
// loaded from the same on-disk revision cannot both commit — the second is
// rejected with a conflict instead of clobbering the first.
func TestJournal_ConcurrentWritersLostUpdateDetected(t *testing.T) {
	root := t.TempDir()
	backend := NewFileBackend(root)

	if err := NewJournal("svc", "dev", "deploy", backend).Save(); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	loadOne := func() *Journal {
		f, err := backend.Load("svc", "dev")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		return NewJournalFromFile(f, backend)
	}

	a := loadOne()
	b := loadOne() // both observe the same revision

	if err := a.Checkpoint("phase", "done"); err != nil {
		t.Fatalf("writer A commit should succeed: %v", err)
	}
	err := b.Checkpoint("phase", "done")
	if _, ok := err.(*errors.ConflictError); !ok {
		t.Fatalf("expected writer B to get a ConflictError (lost update), got %v", err)
	}
}
