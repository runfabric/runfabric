package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/runfabric/runfabric/internal/lease"
	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/state/backends"
)

type LockManager struct {
	Backend   backends.LockBackend
	LeaseFor  time.Duration
	Heartbeat time.Duration
}

// ManagedLock is a held deploy-state lock whose lease is kept alive by the
// shared lease primitive (internal/lease).
type ManagedLock struct {
	Handle *statetypes.Handle
	lease  *lease.Managed
}

func (m *LockManager) Acquire(ctx context.Context, service, stage, operation string) (*ManagedLock, error) {
	handle, err := m.Backend.Acquire(service, stage, operation, m.LeaseFor)
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	return &ManagedLock{
		Handle: handle,
		lease:  lease.Manage(ctx, handle.Renew, m.LeaseFor, m.Heartbeat, handle.Release),
	}, nil
}

func (m *ManagedLock) Release() error {
	if m == nil {
		return nil
	}
	return m.lease.Release()
}

func (m *ManagedLock) HeartbeatErr() <-chan error {
	if m == nil {
		return nil
	}
	return m.lease.HeartbeatErr()
}
