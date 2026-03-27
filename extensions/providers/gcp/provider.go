// Package gcp implements the RunFabric provider for GCP Cloud Functions (Gen 2).
package gcp

// Provider is retained for capability method receivers.
type Provider struct{}

func New() *Provider { return &Provider{} }
