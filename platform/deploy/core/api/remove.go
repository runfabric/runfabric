package api

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// Remove removes the deployment via provider API and deletes the local receipt.
func Remove(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.RemoveResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("remove via API is not supported for unregistered provider %q", provider)
	}
	receipt, err := coreState.LoadReceipt(root, stage)
	if err != nil {
		return &providers.RemoveResult{Provider: provider, Removed: true}, nil
	}
	if receipt.Provider != provider {
		return nil, fmt.Errorf("receipt provider %q does not match config provider %q", receipt.Provider, provider)
	}
	result, err := p.Remove(ctx, cfg, stage, root, toCoreReceipt(receipt))
	if err != nil {
		return nil, err
	}
	_ = coreState.DeleteReceipt(root, stage)
	return result, nil
}

// HasRemover returns whether the provider has an API-based remover.
func HasRemover(provider string) bool {
	return hasProvider(provider)
}
