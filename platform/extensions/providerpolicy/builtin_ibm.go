//go:build !no_builtin_ibm

package providerpolicy

// To move ibm-openwhisk to an external binary plugin:
//   1. Build with -tags no_builtin_ibm  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/ibm-openwhisk/ibm-plugin ./extensions/providers/ibm-openwhisk/cmd/

import (
	ibmprovider "github.com/runfabric/runfabric/extensions/providers/ibm-openwhisk"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "ibm-openwhisk",
			Name:                   "IBM OpenWhisk",
			Description:            "Deploy and run functions on IBM Cloud Functions (OpenWhisk)",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(ibmprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&ibmprovider.Runner{}).Deploy),
			Remove: adaptRemove((&ibmprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&ibmprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&ibmprovider.Logger{}).Logs),
		},
	})
}
