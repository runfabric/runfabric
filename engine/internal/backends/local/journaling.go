package local

import (
	"github.com/runfabric/runfabric/engine/internal/transactions"
)

type JournalBackend struct {
	*transactions.FileBackend
}

func NewJournalBackend(root string) *JournalBackend {
	return &JournalBackend{FileBackend: transactions.NewFileBackend(root)}
}

func (b *JournalBackend) Kind() string {
	return "local"
}
