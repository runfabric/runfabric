package resolution

import (
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/providerpolicy"
)

func loadBuiltinSecretManagers(reg *manifests.PluginRegistry) {
	// Register with credential declarations so plugins list/info and
	// GET /extensions surface the credential contract.
	smCreds := providerpolicy.SecretManagerCredentialVars()
	for _, item := range providerpolicy.BuiltinSecretManagerManifests() {
		reg.Register(&manifests.PluginManifest{
			ID:          item.ID,
			Kind:        manifests.KindSecretManager,
			Name:        item.Name,
			Description: item.Description,
			Credentials: manifests.CredentialSpecs(smCreds[item.ID]),
		})
	}
}
