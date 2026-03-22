package providers

import (
	ext "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

// ErrProviderNotFound returns an error when a provider name is not registered.
func ErrProviderNotFound(name string) error {
	return ext.ErrProviderNotFound(name)
}
