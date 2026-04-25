//go:build !no_builtin_gcp

package providerpolicy

// To move gcp-functions to an external binary plugin:
//   1. Build with -tags no_builtin_gcp  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/gcp-functions/gcp-plugin ./extensions/providers/gcp-functions/cmd/

import (
	gcpprovider "github.com/runfabric/runfabric/extensions/providers/gcp-functions"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "gcp-functions",
			Name:                   "GCP Cloud Functions",
			Description:            "Deploy and run functions on GCP Cloud Functions Gen 2",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream:      adaptPrepareDevStream(gcpprovider.PrepareDevStreamPolicy),
			FetchMetrics:          adaptFetchMetrics(gcpprovider.FetchMetricsPolicy),
			FetchTraces:           adaptFetchTraces(gcpprovider.FetchTracesPolicy),
			Recover:               adaptRecover(gcpprovider.RecoverPolicy),
			SyncOrchestrations:    adaptSyncOrchestrations(gcpprovider.SyncOrchestrationsPolicy),
			RemoveOrchestrations:  adaptRemoveOrchestrations(gcpprovider.RemoveOrchestrationsPolicy),
			InvokeOrchestration:   adaptInvokeOrchestration(gcpprovider.InvokeOrchestrationPolicy),
			InspectOrchestrations: adaptInspectOrchestrations(gcpprovider.InspectOrchestrationsPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&gcpprovider.Runner{}).Deploy),
			Remove: adaptRemove((&gcpprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&gcpprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&gcpprovider.Logger{}).Logs),
		},
	})
}
