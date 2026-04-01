package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinSecretManagers(reg *manifests.PluginRegistry) {
	registerPluginMetaManifests(reg, manifests.KindSecretManager, providerpolicy.BuiltinSecretManagerManifests())
}
