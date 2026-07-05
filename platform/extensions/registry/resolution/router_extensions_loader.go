package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinRouters(reg *manifests.PluginRegistry) providerpolicy.RouterRegistry {
	// Register with the routers' credential declarations (incl. declarative
	// same-cloud provider fallbacks) so plugins list/info and GET /extensions
	// surface the credential contract.
	routerCreds := providerpolicy.RouterPluginCredentialVars()
	for _, item := range providerpolicy.BuiltinRouterManifests() {
		reg.Register(&manifests.PluginManifest{
			ID:          item.ID,
			Kind:        manifests.KindRouter,
			Name:        item.Name,
			Description: item.Description,
			Credentials: manifests.CredentialSpecs(routerCreds[item.ID]),
		})
	}
	return providerpolicy.NewBuiltinRouterRegistry()
}
