//go:build !no_builtin_digitalocean

package providerpolicy

// To move digitalocean-functions to an external binary plugin:
//   1. Build with -tags no_builtin_digitalocean  OR  delete this file.
//   2. Install the binary: go build -o ~/.runfabric/plugins/provider/digitalocean-functions/digitalocean-plugin ./extensions/providers/digitalocean-functions/cmd/

import (
	digitaloceanprovider "github.com/runfabric/runfabric/extensions/providers/digitalocean-functions"
	"github.com/runfabric/runfabric/platform/extensions/inprocess"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy/catalog"
)

func init() {
	registerBuiltin(catalog.ProviderPolicyEntry{
		Descriptor: catalog.ProviderDescriptor{
			ID:                     "digitalocean-functions",
			Name:                   "DigitalOcean Functions",
			Description:            "Deploy and run functions on DigitalOcean App Platform",
			IncludeBuiltinManifest: true,
		},
		Hooks: &inprocess.APIDispatchHooks{
			PrepareDevStream: adaptPrepareDevStream(digitaloceanprovider.PrepareDevStreamPolicy),
		},
		Ops: inprocess.APIOps{
			Deploy: adaptDeploy((&digitaloceanprovider.Runner{}).Deploy),
			Remove: adaptRemove((&digitaloceanprovider.Remover{}).Remove),
			Invoke: adaptInvoke((&digitaloceanprovider.Invoker{}).Invoke),
			Logs:   adaptLogs((&digitaloceanprovider.Logger{}).Logs),
		},
	})
}
