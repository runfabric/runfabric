package locking

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileBackend(t *testing.T) {
	b := NewFileBackend("/tmp")
	if b == nil || b.Root != "/tmp" {
		t.Errorf("NewFileBackend: %+v", b)
	}
}

func TestFileBackend_Acquire_Read_Release(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)
	h, err := b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if h == nil || h.OwnerToken == "" {
		t.Error("handle or token empty")
	}
	rec, err := b.Read("svc", "dev")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rec.Service != "svc" || rec.Stage != "dev" || rec.Operation != "deploy" {
		t.Errorf("Read: %+v", rec)
	}
	if err := h.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	_, err = b.Read("svc", "dev")
	if err == nil {
		t.Error("Read after release should fail")
	}
}

func TestFileBackend_Renew(t *testing.T) {
	root := t.TempDir()
	b := NewFileBackend(root)
	h, err := b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer h.Release()
	if err := b.Renew("svc", "dev", h.OwnerToken, 2*time.Minute); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if err := b.Renew("svc", "dev", "wrong-token", time.Minute); err == nil {
		t.Error("Renew with wrong token should fail")
	}
}

func TestFileBackend_Acquire_ExpiredLockReplaced(t *testing.T) {
	root := t.TempDir()
	lockDir := filepath.Join(root, ".runfabric", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(lockDir, "svc-dev.lock.json")
	stale := `{"service":"svc","stage":"dev","operation":"deploy","ownerToken":"old","pid":1,"createdAt":"2000-01-01T00:00:00Z","expiresAt":"2000-01-01T01:00:00Z"}`
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewFileBackend(root)
	h, err := b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("Acquire (replace stale): %v", err)
	}
	h.Release()
}
