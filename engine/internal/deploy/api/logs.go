package api

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Logs returns logs for the deployed function via provider API.
// If receipt is nil, receipt is loaded from state.Load(root, stage) for backward compatibility.
func Logs(ctx context.Context, provider string, cfg *config.Config, stage, function string, root string, receipt *state.Receipt) (*providers.LogsResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("logs via API is not supported for unregistered provider %q", provider)
	}
	if receipt == nil {
		var err error
		receipt, err = state.Load(root, stage)
		if err != nil {
			return nil, fmt.Errorf("no deployment found for stage %q (run deploy first): %w", stage, err)
		}
	}
	if receipt.Provider != provider {
		return nil, fmt.Errorf("receipt provider %q does not match %q", receipt.Provider, provider)
	}
	return p.Logs(ctx, cfg, stage, function, receipt)
}

// HasLogger returns whether the provider has an API-based logger.
func HasLogger(provider string) bool {
	return hasProvider(provider)
}
