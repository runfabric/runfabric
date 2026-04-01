//go:build !no_builtin_azure

package providerpolicy

// To move azure-functions to an external binary plugin:
//   1. Build with -tags no_builtin_azure  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/azure-functions/azure-plugin ./extensions/providers/azure/cmd/

import (
	azureprovider "github.com/runfabric/runfabric/extensions/providers/azure"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "azure-functions",
			Name:                   "Azure Functions",
			Description:            "Deploy and run functions on Azure Functions",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream:      adaptPrepareDevStream(azureprovider.PrepareDevStreamPolicy),
			FetchMetrics:          adaptFetchMetrics(azureprovider.FetchMetricsPolicy),
			FetchTraces:           adaptFetchTraces(azureprovider.FetchTracesPolicy),
			Recover:               adaptRecover(azureprovider.RecoverPolicy),
			SyncOrchestrations:    adaptSyncOrchestrations(azureprovider.SyncOrchestrationsPolicy),
			RemoveOrchestrations:  adaptRemoveOrchestrations(azureprovider.RemoveOrchestrationsPolicy),
			InvokeOrchestration:   adaptInvokeOrchestration(azureprovider.InvokeOrchestrationPolicy),
			InspectOrchestrations: adaptInspectOrchestrations(azureprovider.InspectOrchestrationsPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&azureprovider.Runner{}).Deploy),
			Remove: adaptRemove((&azureprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&azureprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&azureprovider.Logger{}).Logs),
		},
	})
}
