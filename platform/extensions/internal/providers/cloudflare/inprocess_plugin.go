package cloudflare

import (
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func APIOps() inprocess.APIOps {
	return inprocess.APIOps{
		Deploy: (&Runner{}).Deploy,
		Remove: (&Remover{}).Remove,
		Invoke: (&Invoker{}).Invoke,
		Logs:   (&Logger{}).Logs,
	}
}

func APIHooks() inprocess.APIDispatchHooks {
	return inprocess.APIDispatchHooks{
		PrepareDevStream: PrepareDevStreamPolicy,
	}
}

func NewInProcessPlugin() sdkprovider.Plugin {
	return inprocess.NewAPIOpsTransportPlugin(ProviderID, ProviderName, APIOps(), APIHooks())
}

func PolicyEntry() catalog.ProviderPolicyEntry {
	hooks := APIHooks()
	return catalog.ProviderPolicyEntry{
		Descriptor: Descriptor(),
		Factory:    NewInProcessPlugin,
		Hooks:      &hooks,
	}
}
