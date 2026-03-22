package transactions

import (
	"testing"
)

func TestNewJournal(t *testing.T) {
	root := t.TempDir()
	backend := NewFileBackend(root)
	j := NewJournal("svc", "dev", "deploy", backend)
	if j == nil {
		t.Fatal("NewJournal returned nil")
	}
	f := j.File()
	if f.Service != "svc" || f.Stage != "dev" || f.Operation != "deploy" || f.Status != StatusActive {
		t.Errorf("File(): %+v", f)
	}
	if f.Version != 1 || len(f.Checkpoints) != 0 {
		t.Errorf("version or checkpoints: version=%d checkpoints=%d", f.Version, len(f.Checkpoints))
	}
}

func TestJournal_Record_AndReverse(t *testing.T) {
	root := t.TempDir()
	j := NewJournal("svc", "dev", "deploy", NewFileBackend(root))
	op := Operation{Type: OpCreateLambda, Resource: "fn1", Metadata: map[string]string{"a": "b"}}
	if err := j.Record(op); err != nil {
		t.Fatal(err)
	}
	if err := j.Record(Operation{Type: OpCreateAPI, Resource: "api1"}); err != nil {
		t.Fatal(err)
	}
	rev := j.Reverse()
	if len(rev) != 2 {
		t.Fatalf("Reverse: got %d ops", len(rev))
	}
	if rev[0].Type != OpCreateAPI || rev[1].Type != OpCreateLambda {
		t.Errorf("Reverse order: %+v", rev)
	}
}

func TestJournal_Checkpoint(t *testing.T) {
	root := t.TempDir()
	j := NewJournal("svc", "dev", "deploy", NewFileBackend(root))
	if err := j.Checkpoint("build", "done"); err != nil {
		t.Fatal(err)
	}
	if len(j.File().Checkpoints) != 1 || j.File().Checkpoints[0].Name != "build" || j.File().Checkpoints[0].Status != "done" {
		t.Errorf("Checkpoint: %+v", j.File().Checkpoints)
	}
	if err := j.Checkpoint("build", "updated"); err != nil {
		t.Fatal(err)
	}
	if j.File().Checkpoints[0].Status != "updated" {
		t.Errorf("Checkpoint update: %s", j.File().Checkpoints[0].Status)
	}
}

func TestJournal_MarkRollingBack_MarkRolledBack_MarkCompleted_MarkArchived(t *testing.T) {
	root := t.TempDir()
	j := NewJournal("svc", "dev", "deploy", NewFileBackend(root))
	if err := j.MarkRollingBack(); err != nil {
		t.Fatal(err)
	}
	if j.File().Status != StatusRollingBack {
		t.Errorf("status: %s", j.File().Status)
	}
	if err := j.MarkRolledBack(); err != nil {
		t.Fatal(err)
	}
	if j.File().Status != StatusRolledBack {
		t.Errorf("status: %s", j.File().Status)
	}
	if err := j.MarkCompleted(); err != nil {
		t.Fatal(err)
	}
	if j.File().Status != StatusCompleted {
		t.Errorf("status: %s", j.File().Status)
	}
	if err := j.MarkArchived(); err != nil {
		t.Fatal(err)
	}
	if j.File().Status != StatusArchived {
		t.Errorf("status: %s", j.File().Status)
	}
}

func TestJournal_IncrementAttempt(t *testing.T) {
	root := t.TempDir()
	j := NewJournal("svc", "dev", "deploy", NewFileBackend(root))
	if err := j.IncrementAttempt(); err != nil {
		t.Fatal(err)
	}
	if j.File().AttemptCount != 1 {
		t.Errorf("AttemptCount: %d", j.File().AttemptCount)
	}
}

func TestJournal_Delete(t *testing.T) {
	root := t.TempDir()
	backend := NewFileBackend(root)
	j := NewJournal("svc", "dev", "deploy", backend)
	if err := j.Save(); err != nil {
		t.Fatal(err)
	}
	if err := j.Delete(); err != nil {
		t.Fatal(err)
	}
	_, err := backend.Load("svc", "dev")
	if err == nil {
		t.Fatal("journal should be deleted")
	}
}
