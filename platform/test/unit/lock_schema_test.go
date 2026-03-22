package unit

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/state/locking"
)

func TestLockRecordExtendedFields(t *testing.T) {
	rec := locking.LockRecord{
		Service:         "svc",
		Stage:           "dev",
		Operation:       "deploy",
		OwnerToken:      "owner-123",
		CreatedAt:       "2026-01-01T00:00:00Z",
		ExpiresAt:       "2026-01-01T00:15:00Z",
		LastHeartbeatAt: "2026-01-01T00:01:00Z",
	}

	if rec.OwnerToken == "" || rec.ExpiresAt == "" {
		t.Fatal("expected owner token and expiry")
	}
}
