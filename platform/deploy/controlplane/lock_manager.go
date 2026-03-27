package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/runfabric/runfabric/platform/core/state/backends"
	"github.com/runfabric/runfabric/platform/state/locking"
)

type LockManager struct {
	Backend   backends.LockBackend
	LeaseFor  time.Duration
	Heartbeat time.Duration
}

type ManagedLock struct {
	Handle *locking.Handle
	cancel context.CancelFunc
	errCh  <-chan error
}

func (m *LockManager) Acquire(ctx context.Context, service, stage, operation string) (*ManagedLock, error) {
	handle, err := m.Backend.Acquire(service, stage, operation, m.LeaseFor)
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	hbCtx, cancel := context.WithCancel(ctx)
	errCh := StartHeartbeat(hbCtx, handle, m.LeaseFor, m.Heartbeat)

	return &ManagedLock{
		Handle: handle,
		cancel: cancel,
		errCh:  errCh,
	}, nil
}

func (m *ManagedLock) Release() error {
	if m == nil {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.Handle != nil {
		return m.Handle.Release()
	}
	return nil
}

func (m *ManagedLock) HeartbeatErr() <-chan error {
	if m == nil {
		return nil
	}
	return m.errCh
}
