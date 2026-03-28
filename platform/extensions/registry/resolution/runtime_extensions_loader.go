package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinRuntimes(reg *manifests.PluginRegistry) providerpolicy.RuntimeRegistry {
	for _, rt := range providerpolicy.BuiltinRuntimeManifests() {
		reg.Register(&manifests.PluginManifest{
			ID:          rt.ID,
			Kind:        manifests.KindRuntime,
			Name:        rt.Name,
			Description: rt.Description,
		})
	}
	return providerpolicy.NewBuiltinRuntimeRegistry()
}

func NormalizeRuntimeID(runtime string) string {
	return providerpolicy.NormalizeRuntimeID(runtime)
}
