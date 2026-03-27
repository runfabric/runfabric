package addons

import (
	"fmt"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
)

// ValidateAddonID checks that the addon id is known (optional strict validation).
func ValidateAddonID(registry *manifests.AddonManifestRegistry, id string) error {
	if registry == nil {
		return nil
	}
	if registry.Get(id) == nil {
		return fmt.Errorf("unknown addon %q", id)
	}
	return nil
}
