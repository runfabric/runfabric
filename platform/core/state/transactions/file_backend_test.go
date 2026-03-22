package transactions

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/errors"
)

func TestFileBackend_Load_Save_Delete(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)
	j := &JournalFile{
		Service:   "svc",
		Stage:     "dev",
		Operation: "deploy",
		Status:    StatusActive,
		Version:   1,
	}
	if err := b.Save(j); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.Load("svc", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Service != "svc" || loaded.Stage != "dev" || loaded.Version != 1 {
		t.Errorf("loaded: %+v", loaded)
	}
	if err := b.Delete("svc", "dev"); err != nil {
		t.Fatal(err)
	}
	_, err = b.Load("svc", "dev")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestFileBackend_Load_NotFound(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)
	_, err := b.Load("svc", "dev")
	if err == nil {
		t.Fatal("expected error for missing journal")
	}
}

func TestFileBackend_Save_VersionConflict(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)
	j1 := &JournalFile{Service: "svc", Stage: "dev", Operation: "deploy", Status: StatusActive, Version: 2}
	if err := b.Save(j1); err != nil {
		t.Fatal(err)
	}
	j2 := &JournalFile{Service: "svc", Stage: "dev", Operation: "deploy", Status: StatusActive, Version: 1}
	err := b.Save(j2)
	conflict, ok := err.(*errors.ConflictError)
	if err == nil || !ok {
		t.Fatalf("expected ConflictError, got %v", err)
	}
	if conflict.CurrentVersion != 2 || conflict.IncomingVersion != 1 {
		t.Errorf("ConflictError: current=%d incoming=%d", conflict.CurrentVersion, conflict.IncomingVersion)
	}
}

func TestFileBackend_Delete_NoFile(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)
	if err := b.Delete("svc", "dev"); err != nil {
		t.Errorf("delete of missing file should succeed: %v", err)
	}
}
