package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinRuntimes(reg *manifests.PluginRegistry) providerpolicy.RuntimeRegistry {
	registerPluginMetaManifests(reg, manifests.KindRuntime, providerpolicy.BuiltinRuntimeManifests())
	return providerpolicy.NewBuiltinRuntimeRegistry()
}

func NormalizeRuntimeID(runtime string) string {
	return providerpolicy.NormalizeRuntimeID(runtime)
}
