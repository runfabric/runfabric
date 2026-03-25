package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/state/locking"
)

func TestFileLock(t *testing.T) {
	tmp := t.TempDir()

	lock1, err := locking.Acquire(tmp, "svc", "dev")
	if err != nil {
		t.Fatalf("first lock acquire failed: %v", err)
	}
	defer func() { _ = lock1.Release() }()

	_, err = locking.Acquire(tmp, "svc", "dev")
	if err == nil {
		t.Fatal("expected second lock acquire to fail")
	}

	if err := lock1.Release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}

	_, err = locking.Acquire(tmp, "svc", "dev")
	if err != nil {
		t.Fatalf("expected lock re-acquire after release, got: %v", err)
	}
}
