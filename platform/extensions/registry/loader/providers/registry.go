package providers

import (
	ext "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

// NewRegistry returns an empty provider registry.
func NewRegistry() *Registry {
	return ext.NewRegistry()
}
