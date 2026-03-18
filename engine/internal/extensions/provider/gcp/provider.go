// Package gcp implements the RunFabric provider for GCP Cloud Functions (Gen 2).
package gcp

// Provider implements providers.Provider for GCP Cloud Functions v2.
type Provider struct{}

// New returns a Provider that can be registered with the lifecycle registry.
func New() *Provider {
	return &Provider{}
}

// Name returns the provider identifier (gcp-functions for config compatibility).
func (p *Provider) Name() string {
	return "gcp-functions"
}
