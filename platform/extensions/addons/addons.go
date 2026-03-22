package addons

import coreconfig "github.com/runfabric/runfabric/platform/core/model/config"

// NewBuiltinAddonsCatalog returns the built-in addon catalog, optionally merged
// with entries fetched from the provided addonCatalogURL.
//
// The CLI delegates addon catalog "loading orchestration" here so it stays under
// platform/extensions rather than platform/engine.
func NewBuiltinAddonsCatalog(addonCatalogURL string) []coreconfig.AddonCatalogEntry {
	catalog := coreconfig.AddonCatalog()
	fetched := coreconfig.FetchAddonCatalog(addonCatalogURL)
	if len(fetched) > 0 {
		catalog = coreconfig.MergeAddonCatalogs(catalog, fetched)
	}
	return catalog
}
