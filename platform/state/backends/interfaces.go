package backends

import (
	"time"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
)

type LockBackend interface {
	Acquire(service, stage, operation string, staleAfter time.Duration) (*statetypes.Handle, error)
	Read(service, stage string) (*statetypes.LockRecord, error)
	Release(service, stage string) error
	Kind() string
}

type JournalBackend interface {
	Load(service, stage string) (*statetypes.JournalFile, error)
	Save(j *statetypes.JournalFile) error
	Delete(service, stage string) error
	Kind() string
}

type ReceiptBackend interface {
	Load(stage string) (*statetypes.Receipt, error)
	Save(receipt *statetypes.Receipt) error
	Delete(stage string) error
	ListReleases() ([]statetypes.ReleaseEntry, error)
	Kind() string
}
