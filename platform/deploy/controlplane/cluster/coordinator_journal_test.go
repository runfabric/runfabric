package cluster

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

// TestResumeOrNewJournal_ResumesActiveJournal guards the regression where
// AcquireRunContext unconditionally created a fresh journal, overwriting an
// active journal from a crashed run and destroying its checkpoints/recovery
// state.
func TestResumeOrNewJournal_ResumesActiveJournal(t *testing.T) {
	root := t.TempDir()
	backend := transactions.NewFileBackend(root)

	// Seed an active journal from a prior (crashed) "deploy" run with a checkpoint.
	seed := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := seed.Checkpoint("deploy_functions", "done"); err != nil {
		t.Fatalf("seed checkpoint: %v", err)
	}

	// A subsequent operation must resume the existing journal, not overwrite it.
	j, err := resumeOrNewJournal(backend, "svc", "dev", "remove")
	if err != nil {
		t.Fatalf("resumeOrNewJournal: %v", err)
	}
	f := j.File()
	if f.Operation != "deploy" {
		t.Errorf("operation = %q, want deploy (existing journal must be preserved, not replaced with the new op)", f.Operation)
	}
	if len(f.Checkpoints) != 1 || f.Checkpoints[0].Name != "deploy_functions" {
		t.Errorf("checkpoints = %+v, want the seeded deploy_functions checkpoint preserved", f.Checkpoints)
	}
}

// TestResumeOrNewJournal_CreatesWhenAbsent verifies a fresh journal is created
// and persisted when none exists.
func TestResumeOrNewJournal_CreatesWhenAbsent(t *testing.T) {
	root := t.TempDir()
	backend := transactions.NewFileBackend(root)

	j, err := resumeOrNewJournal(backend, "svc", "dev", "deploy")
	if err != nil {
		t.Fatalf("resumeOrNewJournal: %v", err)
	}
	if j.File().Operation != "deploy" || j.File().Status != transactions.StatusActive {
		t.Errorf("new journal = %+v, want operation=deploy status=active", j.File())
	}
	if loaded, err := backend.Load("svc", "dev"); err != nil || loaded == nil {
		t.Fatalf("new journal was not persisted: loaded=%v err=%v", loaded, err)
	}
}

// TestResumeOrNewJournal_StartsFreshAfterTerminalJournal verifies a completed
// (terminal) journal is not resumed — a new run starts fresh.
func TestResumeOrNewJournal_StartsFreshAfterTerminalJournal(t *testing.T) {
	root := t.TempDir()
	backend := transactions.NewFileBackend(root)

	seed := transactions.NewJournal("svc", "dev", "deploy", backend)
	if err := seed.Checkpoint("deploy_functions", "done"); err != nil {
		t.Fatalf("seed checkpoint: %v", err)
	}
	if err := seed.MarkCompleted(); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	j, err := resumeOrNewJournal(backend, "svc", "dev", "remove")
	if err != nil {
		t.Fatalf("resumeOrNewJournal: %v", err)
	}
	if j.File().Operation != "remove" {
		t.Errorf("operation = %q, want remove (terminal journal must not be resumed)", j.File().Operation)
	}
	if len(j.File().Checkpoints) != 0 {
		t.Errorf("checkpoints = %+v, want empty fresh journal", j.File().Checkpoints)
	}
}
