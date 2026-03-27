package core

import (
	"strconv"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// EnrichReceiptWithWorkflows sets receipt metadata for workflow graph/hash enrichment when workflows are enabled.
// Call before saving the receipt so deploy receipts carry workflow version for observability.
func EnrichReceiptWithWorkflows(receipt *Receipt, cfg *config.Config) {
	if receipt == nil || cfg == nil {
		return
	}
	compiled, err := config.CompileWorkflowGraphFromConfig(cfg)
	if receipt.Metadata == nil {
		receipt.Metadata = make(map[string]string)
	}
	if err != nil {
		receipt.Metadata["workflow.graph.error"] = err.Error()
		return
	}
	if compiled == nil {
		delete(receipt.Metadata, "workflow.graph.hash")
		delete(receipt.Metadata, "workflow.graph.entrypoint")
		delete(receipt.Metadata, "workflow.graph.nodes")
		delete(receipt.Metadata, "workflow.graph.edges")
		return
	}
	receipt.Metadata["workflow.graph.hash"] = compiled.Hash
	receipt.Metadata["workflow.graph.entrypoint"] = compiled.Entrypoint
	receipt.Metadata["workflow.graph.nodes"] = strconv.Itoa(len(compiled.Nodes))
	receipt.Metadata["workflow.graph.edges"] = strconv.Itoa(len(compiled.Edges))
	delete(receipt.Metadata, "workflow.graph.error")
}
