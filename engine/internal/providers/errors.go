package providers

import (
	ext "github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// ErrProviderNotFound returns an error when a provider name is not registered.
func ErrProviderNotFound(name string) error {
	return ext.ErrProviderNotFound(name)
}
