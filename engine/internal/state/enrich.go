package state

import (
	"github.com/runfabric/runfabric/engine/internal/config"
)

// EnrichReceiptWithAiWorkflow sets receipt.Metadata with aiWorkflow hash and entrypoint when cfg has aiWorkflow enabled (Phase 14.3).
// Call before saving the receipt so deploy receipts carry workflow version for observability.
func EnrichReceiptWithAiWorkflow(receipt *Receipt, cfg *config.Config) {
	if receipt == nil || cfg == nil || cfg.AiWorkflow == nil || !cfg.AiWorkflow.Enable {
		return
	}
	g, err := config.CompileAiWorkflow(cfg.AiWorkflow)
	if err != nil || g == nil {
		return
	}
	if receipt.Metadata == nil {
		receipt.Metadata = make(map[string]string)
	}
	receipt.Metadata["aiWorkflowHash"] = g.Hash
	receipt.Metadata["aiWorkflowEntrypoint"] = g.Entrypoint
}
