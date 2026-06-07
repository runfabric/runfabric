package locking

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestAcquire_LockFileOwnerOnly verifies the lock file (which embeds the secret
// OwnerToken) is created owner-only so other local users cannot read the token
// and hijack the lock.
func TestAcquire_LockFileOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes not enforced on windows")
	}
	root := t.TempDir()
	b := NewFileBackend(root)
	if _, err := b.Acquire("svc", "dev", "deploy", time.Minute); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	lockFile := filepath.Join(root, ".runfabric", "locks", "svc-dev.lock.json")
	fi, err := os.Stat(lockFile)
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("lock file perm = %o, want 600", perm)
	}

	di, err := os.Stat(filepath.Dir(lockFile))
	if err != nil {
		t.Fatalf("stat lock dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("lock dir perm = %o, want 700", perm)
	}
}
