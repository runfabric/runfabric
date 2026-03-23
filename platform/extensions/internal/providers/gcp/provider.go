// Package gcp implements the RunFabric provider for GCP Cloud Functions (Gen 2).
package gcp

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

// Provider implements ProviderPlugin for GCP Cloud Functions v2.
type Provider struct{}

// New returns a Provider that can be registered with the lifecycle registry.
func New() *Provider {
	return &Provider{}
}

func (p *Provider) Meta() providers.ProviderMeta {
	return providers.ProviderMeta{
		Name:            p.Name(),
		Capabilities:    []string{"deploy", "remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (p *Provider) ValidateConfig(ctx context.Context, req providers.ValidateConfigRequest) error {
	return nil
}

// Name returns the provider identifier (gcp-functions for config compatibility).
func (p *Provider) Name() string {
	return ProviderID
}
