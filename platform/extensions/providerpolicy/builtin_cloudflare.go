//go:build !no_builtin_cloudflare

package providerpolicy

// To move cloudflare-workers to an external binary plugin:
//   1. Build with -tags no_builtin_cloudflare  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/cloudflare-workers/cloudflare-plugin ./extensions/providers/cloudflare-workers/cmd/

import (
	cfprovider "github.com/runfabric/runfabric/extensions/providers/cloudflare-workers"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "cloudflare-workers",
			Name:                   "Cloudflare Workers",
			Description:            "Deploy and run functions on Cloudflare Workers",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(cfprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&cfprovider.Runner{}).Deploy),
			Remove: adaptRemove((&cfprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&cfprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&cfprovider.Logger{}).Logs),
		},
	})
}
