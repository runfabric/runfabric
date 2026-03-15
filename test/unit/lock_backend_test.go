package unit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/runfabric/runfabric/internal/locking"
)

func TestLockBackendAcquireRelease(t *testing.T) {
	tmp := t.TempDir()
	b := locking.NewFileBackend(tmp)

	h, err := b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	_, err = b.Acquire("svc", "dev", "deploy", time.Minute)
	if err == nil {
		t.Fatal("expected second acquire to fail")
	}

	if err := h.Release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}

	_, err = b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("expected acquire after release to succeed: %v", err)
	}
}

func TestStaleLockReplaced(t *testing.T) {
	tmp := t.TempDir()
	b := locking.NewFileBackend(tmp)

	lockDir := filepath.Join(tmp, ".runfabric", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(lockDir, "svc-dev.lock.json")
	stale := `{
	  "service": "svc",
	  "stage": "dev",
	  "operation": "deploy",
	  "pid": 999999,
	  "createdAt": "2000-01-01T00:00:00Z",
	  "expiresAt": "2000-01-01T01:00:00Z"
	}`
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("expected stale lock replacement, got: %v", err)
	}
}
