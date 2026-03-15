package local

import (
	"github.com/runfabric/runfabric/internal/locking"
)

type LockBackend struct {
	*locking.FileBackend
}

func NewLockBackend(root string) *LockBackend {
	return &LockBackend{FileBackend: locking.NewFileBackend(root)}
}

func (b *LockBackend) Kind() string {
	return "local"
}
