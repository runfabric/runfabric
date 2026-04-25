package api

import (
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// CoreStateConnector is the deploy->core boundary for receipt/state operations.
type CoreStateConnector interface {
	LoadReceipt(root, stage string) (*ReceiptRecord, error)
	SaveReceipt(root string, receipt *ReceiptRecord) error
	DeleteReceipt(root, stage string) error
	EnrichReceiptWithWorkflows(receipt *ReceiptRecord, cfg *config.Config)
}

type coreStateAdapter struct{}

func (coreStateAdapter) LoadReceipt(root, stage string) (*ReceiptRecord, error) {
	r, err := state.Load(root, stage)
	if err != nil {
		return nil, err
	}
	return toReceiptRecord(r), nil
}

func (coreStateAdapter) SaveReceipt(root string, receipt *ReceiptRecord) error {
	return state.Save(root, toCoreReceipt(receipt))
}

func (coreStateAdapter) DeleteReceipt(root, stage string) error {
	return state.Delete(root, stage)
}

func (coreStateAdapter) EnrichReceiptWithWorkflows(receipt *ReceiptRecord, cfg *config.Config) {
	coreReceipt := toCoreReceipt(receipt)
	state.EnrichReceiptWithWorkflows(coreReceipt, cfg)
	receipt.Metadata = cloneStringMap(coreReceipt.Metadata)
}

var coreState CoreStateConnector = coreStateAdapter{}
