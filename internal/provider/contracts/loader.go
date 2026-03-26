package contracts

import provider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"

// NewNamedPlugin returns a ProviderPlugin that delegates to p but reports Meta().Name as name.
var NewNamedPlugin = provider.NewNamedPlugin
