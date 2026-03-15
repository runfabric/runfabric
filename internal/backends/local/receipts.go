package local

import (
	"fmt"

	"github.com/runfabric/runfabric/internal/state"
)

type ReceiptBackend struct {
	Root string
}

func NewReceiptBackend(root string) *ReceiptBackend {
	return &ReceiptBackend{Root: root}
}

func (b *ReceiptBackend) Load(stage string) (*state.Receipt, error) {
	return state.Load(b.Root, stage)
}

func (b *ReceiptBackend) Save(receipt *state.Receipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}
	return state.Save(b.Root, receipt)
}

func (b *ReceiptBackend) Delete(stage string) error {
	return state.Delete(b.Root, stage)
}

func (b *ReceiptBackend) Kind() string {
	return "local"
}
