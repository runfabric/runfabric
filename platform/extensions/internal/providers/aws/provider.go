package aws

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

type Provider struct{}

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

func (p *Provider) Name() string {
	return ProviderID
}
