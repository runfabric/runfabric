package integration

import (
	"testing"

	provider "github.com/runfabric/runfabric/internal/provider/contracts"
)

func resolveAWSProvider(t *testing.T) provider.ProviderPlugin {
	return resolveTestProvider(t)
}
