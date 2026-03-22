package unit

import (
	"testing"
	"time"

	"github.com/runfabric/runfabric/platform/core/state/locking"
)

func TestFileLockOwnerToken(t *testing.T) {
	tmp := t.TempDir()
	b := locking.NewFileBackend(tmp)

	h, err := b.Acquire("svc", "dev", "deploy", time.Minute)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	defer func() { _ = h.Release() }()

	if h.OwnerToken == "" {
		t.Fatal("expected non-empty owner token")
	}
}
