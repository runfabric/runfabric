package controlplane

import (
	"strings"
	"sync"
)

// CostTracker records per-provider model token usage and approximate USD cost.
// Provider-specific implementations apply published pricing as of 2024.
type CostTracker interface {
	RecordCost(provider, model string, inputTokens, outputTokens int)
	TotalCostUSD() float64
	Summary() map[string]any
}

// NoopCostTracker discards all events. Used as the default.
type NoopCostTracker struct{}

func (NoopCostTracker) RecordCost(_, _ string, _, _ int) {}
func (NoopCostTracker) TotalCostUSD() float64            { return 0 }
func (NoopCostTracker) Summary() map[string]any          { return map[string]any{} }

// costRecord holds one recorded model invocation.
type costRecord struct {
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// InMemoryCostTracker accumulates cost records in memory.
// Embed this in provider-specific trackers for shared ledger logic.
type InMemoryCostTracker struct {
	mu      sync.Mutex
	records []costRecord
}

func (t *InMemoryCostTracker) record(provider, model string, inputTokens, outputTokens int, costPerInput, costPerOutput float64) {
	cost := float64(inputTokens)*costPerInput + float64(outputTokens)*costPerOutput
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, costRecord{
		Provider:     provider,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      cost,
	})
}

func (t *InMemoryCostTracker) TotalCostUSD() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var total float64
	for _, r := range t.records {
		total += r.CostUSD
	}
	return total
}

func (t *InMemoryCostTracker) Summary() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := 0.0
	perProvider := map[string]float64{}
	for _, r := range t.records {
		total += r.CostUSD
		perProvider[r.Provider] += r.CostUSD
	}
	return map[string]any{
		"totalCostUSD": total,
		"perProvider":  perProvider,
		"recordCount":  len(t.records),
	}
}

// AWSCostTracker records AWS Bedrock per-token costs.
// Pricing: Claude 3 Sonnet ~$3/$15 per 1M tokens; Haiku ~$0.25/$1.25 per 1M tokens (2024).
type AWSCostTracker struct {
	InMemoryCostTracker
}

func (t *AWSCostTracker) RecordCost(_, model string, inputTokens, outputTokens int) {
	var costIn, costOut float64
	if strings.Contains(model, "sonnet") {
		costIn, costOut = 0.000003, 0.000015 // $3.00/$15.00 per 1M tokens
	} else {
		costIn, costOut = 0.00000025, 0.00000125 // Haiku: $0.25/$1.25 per 1M tokens
	}
	t.record("aws", model, inputTokens, outputTokens, costIn, costOut)
}

// GCPCostTracker records GCP Vertex AI per-token costs.
// Pricing: Gemini 1.5 Pro ~$1.25/$5 per 1M tokens; Flash ~$0.075/$0.30 per 1M (2024).
type GCPCostTracker struct {
	InMemoryCostTracker
}

func (t *GCPCostTracker) RecordCost(_, model string, inputTokens, outputTokens int) {
	var costIn, costOut float64
	if strings.Contains(model, "flash") {
		costIn, costOut = 0.000000075, 0.0000003 // Flash pricing
	} else {
		costIn, costOut = 0.00000125, 0.000005 // Pro pricing
	}
	t.record("gcp", model, inputTokens, outputTokens, costIn, costOut)
}

// AzureCostTracker records Azure OpenAI per-token costs.
// Pricing: GPT-4o ~$5/$15 per 1M; GPT-4o-mini ~$0.15/$0.60 per 1M tokens (2024).
type AzureCostTracker struct {
	InMemoryCostTracker
}

func (t *AzureCostTracker) RecordCost(_, model string, inputTokens, outputTokens int) {
	var costIn, costOut float64
	if strings.Contains(model, "mini") {
		costIn, costOut = 0.00000015, 0.0000006 // GPT-4o-mini
	} else {
		costIn, costOut = 0.000005, 0.000015 // GPT-4o
	}
	t.record("azure", model, inputTokens, outputTokens, costIn, costOut)
}

// ProviderCostTracker returns the appropriate CostTracker for a cloud provider.
// Falls back to NoopCostTracker for unknown providers.
func ProviderCostTracker(provider string) CostTracker {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aws":
		return &AWSCostTracker{}
	case "gcp":
		return &GCPCostTracker{}
	case "azure":
		return &AzureCostTracker{}
	default:
		return NoopCostTracker{}
	}
}
