package providers

import (
	ext "github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// NewNamedProvider returns a Provider that delegates to p but reports Name() as name.
func NewNamedProvider(name string, p Provider) Provider {
	return ext.NewNamedProvider(name, p)
}
