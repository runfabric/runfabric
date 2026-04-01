package catalog

import (
	"context"
	"sort"
	"sync"
	"time"

	statetypes "github.com/runfabric/runfabric/internal/state/types"
)

type StateBackendOptions struct {
	Root            string
	AWSRegion       string
	S3Bucket        string
	S3Prefix        string
	DynamoTableName string
	PostgresDSN     string
	PostgresTable   string
	SqlitePath      string
	ReceiptTable    string
}

type StateLockBackend interface {
	Acquire(service, stage, operation string, staleAfter time.Duration) (*statetypes.Handle, error)
	Read(service, stage string) (*statetypes.LockRecord, error)
	Release(service, stage string) error
	Kind() string
}

type StateJournalBackend interface {
	Load(service, stage string) (*statetypes.JournalFile, error)
	Save(j *statetypes.JournalFile) error
	Delete(service, stage string) error
	Kind() string
}

type StateReceiptBackend interface {
	Load(stage string) (*statetypes.Receipt, error)
	Save(receipt *statetypes.Receipt) error
	Delete(stage string) error
	ListReleases() ([]statetypes.ReleaseEntry, error)
	Kind() string
}

type StateBundleComponents struct {
	Locks    StateLockBackend
	Journals StateJournalBackend
	Receipts StateReceiptBackend
}

type StateBundleComponentsFactory func(ctx context.Context, opts StateBackendOptions) (*StateBundleComponents, error)

var (
	stateFactoryMu sync.RWMutex
	stateFactories = map[string]StateBundleComponentsFactory{}
)

func RegisterStateBackendFactory(kind string, factory StateBundleComponentsFactory) {
	if factory == nil {
		return
	}
	stateFactoryMu.Lock()
	stateFactories[kind] = factory
	stateFactoryMu.Unlock()
}

func StateBackendFactory(kind string) (StateBundleComponentsFactory, bool) {
	stateFactoryMu.RLock()
	factory, ok := stateFactories[kind]
	stateFactoryMu.RUnlock()
	return factory, ok
}

func StateBackendKinds() []string {
	stateFactoryMu.RLock()
	kinds := make([]string, 0, len(stateFactories))
	for kind := range stateFactories {
		kinds = append(kinds, kind)
	}
	stateFactoryMu.RUnlock()
	sort.Strings(kinds)
	return kinds
}
