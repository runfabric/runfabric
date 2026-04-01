//go:build !no_builtin_aws

package providerpolicy

// To move aws-lambda to an external binary plugin:
//   1. Build with -tags no_builtin_aws  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/aws-lambda/aws-plugin ./extensions/providers/aws/cmd/
//   3. The plugin.yaml in extensions/providers/aws/ defines the manifest.

import (
	awsprovider "github.com/runfabric/runfabric/extensions/providers/aws"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "aws-lambda",
			Name:                   "AWS Lambda",
			Description:            "Deploy and run functions on AWS Lambda",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream:      adaptPrepareDevStream(awsprovider.PrepareDevStreamPolicy),
			FetchMetrics:          adaptFetchMetrics(awsprovider.FetchMetricsPolicy),
			FetchTraces:           adaptFetchTraces(awsprovider.FetchTracesPolicy),
			Recover:               adaptRecover(awsprovider.RecoverPolicy),
			SyncOrchestrations:    adaptSyncOrchestrations(awsprovider.SyncOrchestrationsPolicy),
			RemoveOrchestrations:  adaptRemoveOrchestrations(awsprovider.RemoveOrchestrationsPolicy),
			InvokeOrchestration:   adaptInvokeOrchestration(awsprovider.InvokeOrchestrationPolicy),
			InspectOrchestrations: adaptInspectOrchestrations(awsprovider.InspectOrchestrationsPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy(awsprovider.DeployAPIOps),
			Remove: adaptRemove(awsprovider.RemoveAPIOps),
			Invoke: adaptInvoke(awsprovider.InvokeAPIOps),
			Logs:   adaptLogs(awsprovider.LogsAPIOps),
		},
	})
}
