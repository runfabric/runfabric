package backends

import (
	"time"

	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/locking"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

type LockBackend interface {
	Acquire(service, stage, operation string, staleAfter time.Duration) (*locking.Handle, error)
	Read(service, stage string) (*locking.LockRecord, error)
	Release(service, stage string) error
	Kind() string
}

type JournalBackend interface {
	Load(service, stage string) (*transactions.JournalFile, error)
	Save(j *transactions.JournalFile) error
	Delete(service, stage string) error
	Kind() string
}

type ReceiptBackend interface {
	Load(stage string) (*state.Receipt, error)
	Save(receipt *state.Receipt) error
	Delete(stage string) error
	ListReleases() ([]state.ReleaseEntry, error)
	Kind() string
}
