//go:build !no_builtin_vercel

package providerpolicy

// To move vercel to an external binary plugin:
//   1. Build with -tags no_builtin_vercel  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/vercel/vercel-plugin ./extensions/providers/vercel/cmd/

import (
	vercelprovider "github.com/runfabric/runfabric/extensions/providers/vercel"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "vercel",
			Name:                   "Vercel",
			Description:            "Deploy and run functions on Vercel Serverless",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(vercelprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&vercelprovider.Runner{}).Deploy),
			Remove: adaptRemove((&vercelprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&vercelprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&vercelprovider.Logger{}).Logs),
		},
	})
}
