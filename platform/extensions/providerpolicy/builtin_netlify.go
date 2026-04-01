//go:build !no_builtin_netlify

package providerpolicy

// To move netlify to an external binary plugin:
//   1. Build with -tags no_builtin_netlify  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/netlify/netlify-plugin ./extensions/providers/netlify/cmd/

import (
	netlifyprovider "github.com/runfabric/runfabric/extensions/providers/netlify"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "netlify",
			Name:                   "Netlify",
			Description:            "Deploy and run functions on Netlify Functions",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(netlifyprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&netlifyprovider.Runner{}).Deploy),
			Remove: adaptRemove((&netlifyprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&netlifyprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&netlifyprovider.Logger{}).Logs),
		},
	})
}
