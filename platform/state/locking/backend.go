package locking

import (
	"time"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
)

// Backend and LeaseBackend operate on the shared lock contract types in
// internal/state/types (Handle, LockRecord) — the same types the deploy-state
// lock backends use — so a file lock and a remote lock are interchangeable at
// the interface level.
type Backend interface {
	Acquire(service, stage, operation string, staleAfter time.Duration) (*statetypes.Handle, error)
	Read(service, stage string) (*statetypes.LockRecord, error)
	Release(service, stage string) error
}

type LeaseBackend interface {
	Backend
	Renew(service, stage, ownerToken string, leaseFor time.Duration) error
	Steal(service, stage, newOperation string, staleAfter time.Duration) (*statetypes.Handle, error)
	ReleaseOwned(service, stage, ownerToken string) error
}
