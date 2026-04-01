//go:build !no_builtin_kubernetes

package providerpolicy

// To move kubernetes to an external binary plugin:
//   1. Build with -tags no_builtin_kubernetes  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/kubernetes/kubernetes-plugin ./extensions/providers/kubernetes/cmd/

import (
	kubernetesprovider "github.com/runfabric/runfabric/extensions/providers/kubernetes"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "kubernetes",
			Name:                   "Kubernetes",
			Description:            "Deploy and run functions on Kubernetes",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(kubernetesprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&kubernetesprovider.Runner{}).Deploy),
			Remove: adaptRemove((&kubernetesprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&kubernetesprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&kubernetesprovider.Logger{}).Logs),
		},
	})
}
