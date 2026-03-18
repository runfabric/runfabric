package addons

import (
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
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
