package controlplane

import (
	"context"
	"time"

	"github.com/runfabric/runfabric/internal/backends"
	"github.com/runfabric/runfabric/internal/transactions"
)

type Coordinator struct {
	Locks    backends.LockBackend
	Journals backends.JournalBackend
	Receipts backends.ReceiptBackend

	LeaseFor  time.Duration
	Heartbeat time.Duration
}

type RunContext struct {
	Lock    *ManagedLock
	Journal *transactions.Journal
}

func (c *Coordinator) AcquireRunContext(
	ctx context.Context,
	service, stage, operation string,
) (*RunContext, error) {
	lockMgr := &LockManager{
		Backend:   c.Locks,
		LeaseFor:  c.LeaseFor,
		Heartbeat: c.Heartbeat,
	}

	lock, err := lockMgr.Acquire(ctx, service, stage, operation)
	if err != nil {
		return nil, err
	}

	journal := transactions.NewJournal(service, stage, operation, c.Journals)
	if err := journal.Save(); err != nil {
		_ = lock.Release()
		return nil, err
	}

	return &RunContext{
		Lock:    lock,
		Journal: journal,
	}, nil
}

func (c *Coordinator) Close(run *RunContext) error {
	if run == nil {
		return nil
	}
	if run.Lock != nil {
		return run.Lock.Release()
	}
	return nil
}
