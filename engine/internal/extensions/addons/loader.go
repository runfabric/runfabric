package addons

import (
	"github.com/runfabric/runfabric/engine/internal/config"
)

// CatalogEntry is a single addon catalog entry (name, version, description).
type CatalogEntry struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// LoadBuiltinCatalog returns the built-in addon catalog (used by runfabric addons list).
func LoadBuiltinCatalog() []CatalogEntry {
	entries := config.AddonCatalog()
	out := make([]CatalogEntry, len(entries))
	for i, e := range entries {
		out[i] = CatalogEntry{Name: e.Name, Version: e.Version, Description: e.Description}
	}
	return out
}
