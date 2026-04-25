package aws

import (
	"context"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Meta() sdkprovider.Meta {
	return sdkprovider.Meta{
		Name:            p.Name(),
		Capabilities:    []string{"remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (p *Provider) ValidateConfig(ctx context.Context, req sdkprovider.ValidateConfigRequest) error {
	return nil
}

func (p *Provider) Name() string {
	return ProviderID
}
