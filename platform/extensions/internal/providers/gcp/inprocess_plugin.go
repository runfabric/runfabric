package gcp

import (
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func NewInProcessPlugin() sdkprovider.Plugin {
	return NewTransportPlugin()
}

func PolicyEntry() catalog.ProviderPolicyEntry {
	return catalog.ProviderPolicyEntry{
		Descriptor: Descriptor(),
		Factory:    NewInProcessPlugin,
	}
}
