package api

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// Invoke invokes the deployed function via provider API.
func Invoke(ctx context.Context, provider string, cfg *config.Config, stage, function string, payload []byte, root string) (*providers.InvokeResult, error) {
	p, ok := getProvider(provider)
	if !ok {
		return nil, fmt.Errorf("invoke via API is not supported for unregistered provider %q", provider)
	}
	receipt, err := state.Load(root, stage)
	if err != nil {
		return nil, fmt.Errorf("no deployment found for stage %q (run deploy first): %w", stage, err)
	}
	if receipt.Provider != provider {
		return nil, fmt.Errorf("receipt provider %q does not match %q", receipt.Provider, provider)
	}
	return p.Invoke(ctx, cfg, stage, function, payload, receipt)
}

// HasInvoker returns whether the provider has an API-based invoker.
func HasInvoker(provider string) bool {
	return hasProvider(provider)
}
