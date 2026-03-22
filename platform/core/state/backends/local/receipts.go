package local

import (
	"fmt"

	state "github.com/runfabric/runfabric/platform/core/state/core"
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

func (b *ReceiptBackend) ListReleases() ([]state.ReleaseEntry, error) {
	return state.ListReleases(b.Root)
}

func (b *ReceiptBackend) Kind() string {
	return "local"
}
