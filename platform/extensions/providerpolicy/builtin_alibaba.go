//go:build !no_builtin_alibaba

package providerpolicy

// To move alibaba-fc to an external binary plugin:
//   1. Build with -tags no_builtin_alibaba  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/alibaba-fc/alibaba-plugin ./extensions/providers/alibaba/cmd/

import (
	alibabaprovider "github.com/runfabric/runfabric/extensions/providers/alibaba"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "alibaba-fc",
			Name:                   "Alibaba FC",
			Description:            "Deploy and run functions on Alibaba Cloud Function Compute",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(alibabaprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&alibabaprovider.Runner{}).Deploy),
			Remove: adaptRemove((&alibabaprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&alibabaprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&alibabaprovider.Logger{}).Logs),
		},
	})
}
