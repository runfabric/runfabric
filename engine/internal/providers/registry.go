package providers

import (
	ext "github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// NewRegistry returns an empty provider registry.
func NewRegistry() *Registry {
	return ext.NewRegistry()
}
