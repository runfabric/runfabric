package cluster

import (
	"context"
	"time"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	"github.com/runfabric/runfabric/platform/state/backends"
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

type journalBackendAdapter struct {
	backend backends.JournalBackend
}

func (a journalBackendAdapter) Load(service, stage string) (*transactions.JournalFile, error) {
	jf, err := a.backend.Load(service, stage)
	if err != nil || jf == nil {
		return nil, err
	}
	out := &transactions.JournalFile{
		Service:       jf.Service,
		Stage:         jf.Stage,
		Operation:     jf.Operation,
		Status:        transactions.Status(jf.Status),
		StartedAt:     jf.StartedAt,
		UpdatedAt:     jf.UpdatedAt,
		Version:       jf.Version,
		AttemptCount:  jf.AttemptCount,
		LastAttemptAt: jf.LastAttemptAt,
		Checksum:      jf.Checksum,
	}
	if len(jf.Checkpoints) > 0 {
		out.Checkpoints = make([]transactions.JournalCheckpoint, 0, len(jf.Checkpoints))
		for _, cp := range jf.Checkpoints {
			out.Checkpoints = append(out.Checkpoints, transactions.JournalCheckpoint{Name: cp.Name, Status: cp.Status})
		}
	}
	if len(jf.Operations) > 0 {
		out.Operations = make([]transactions.Operation, 0, len(jf.Operations))
		for _, op := range jf.Operations {
			out.Operations = append(out.Operations, transactions.Operation{
				Type:     transactions.OperationType(op.Type),
				Resource: op.Resource,
				Metadata: op.Metadata,
			})
		}
	}
	return out, nil
}

func (a journalBackendAdapter) Save(j *transactions.JournalFile) error {
	if j == nil {
		return a.backend.Save(nil)
	}
	out := &statetypes.JournalFile{
		Service:       j.Service,
		Stage:         j.Stage,
		Operation:     j.Operation,
		Status:        statetypes.Status(j.Status),
		StartedAt:     j.StartedAt,
		UpdatedAt:     j.UpdatedAt,
		Version:       j.Version,
		AttemptCount:  j.AttemptCount,
		LastAttemptAt: j.LastAttemptAt,
		Checksum:      j.Checksum,
	}
	if len(j.Checkpoints) > 0 {
		out.Checkpoints = make([]statetypes.JournalCheckpoint, 0, len(j.Checkpoints))
		for _, cp := range j.Checkpoints {
			out.Checkpoints = append(out.Checkpoints, statetypes.JournalCheckpoint{Name: cp.Name, Status: cp.Status})
		}
	}
	if len(j.Operations) > 0 {
		out.Operations = make([]statetypes.Operation, 0, len(j.Operations))
		for _, op := range j.Operations {
			out.Operations = append(out.Operations, statetypes.Operation{
				Type:     statetypes.OperationType(op.Type),
				Resource: op.Resource,
				Metadata: op.Metadata,
			})
		}
	}
	return a.backend.Save(out)
}

func (a journalBackendAdapter) Delete(service, stage string) error {
	return a.backend.Delete(service, stage)
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

	journal := transactions.NewJournal(service, stage, operation, journalBackendAdapter{backend: c.Journals})
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
