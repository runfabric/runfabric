//go:build !no_builtin_fly

package providerpolicy

// To move fly-machines to an external binary plugin:
//   1. Build with -tags no_builtin_fly  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/fly-machines/fly-plugin ./extensions/providers/fly-machines/cmd/

import (
	flyprovider "github.com/runfabric/runfabric/extensions/providers/fly-machines"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "fly-machines",
			Name:                   "Fly.io Machines",
			Description:            "Deploy and run functions on Fly.io Machines",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(flyprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&flyprovider.Runner{}).Deploy),
			Remove: adaptRemove((&flyprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&flyprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&flyprovider.Logger{}).Logs),
		},
	})
}
