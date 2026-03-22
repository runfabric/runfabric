package resolution

import (
	"strings"

	"github.com/runfabric/runfabric/platform/extension/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
)

// PluginCatalog is the merged built-in + external plugin view used by CLI list/info/search.
type PluginCatalog struct {
	Registry *manifests.PluginRegistry
	Invalid  []external.InvalidPlugin
}

// DiscoverPluginCatalog returns built-in plugin manifests merged with discovered external plugins.
// Built-ins keep precedence unless discover options explicitly prefer external manifests.
func DiscoverPluginCatalog(opts external.DiscoverOptions) (*PluginCatalog, error) {
	reg := manifests.NewPluginRegistry()
	res, err := external.Discover(opts)
	if err != nil {
		return &PluginCatalog{Registry: reg}, err
	}
	mergePluginManifests(reg, res.Plugins, opts.PreferExternal)
	return &PluginCatalog{Registry: reg, Invalid: res.Invalid}, nil
}

// HasInstalledExternalPlugin reports whether the requested external plugin kind/id[/version] exists on disk.
func HasInstalledExternalPlugin(kind manifests.PluginKind, id, version string) (bool, error) {
	id = strings.TrimSpace(id)
	version = strings.TrimSpace(version)
	if id == "" {
		return false, nil
	}
	pins := map[string]string{}
	if version != "" {
		pins[id] = version
	}
	res, err := external.Discover(external.DiscoverOptions{PinnedVersions: pins})
	if err != nil {
		return false, err
	}
	for _, m := range res.Plugins {
		if m == nil || m.Source != "external" {
			continue
		}
		if m.Kind != kind || !strings.EqualFold(m.ID, id) {
			continue
		}
		if version != "" && !strings.EqualFold(strings.TrimSpace(m.Version), version) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func mergePluginManifests(reg *manifests.PluginRegistry, discovered []*manifests.PluginManifest, preferExternal bool) {
	for _, m := range discovered {
		if m == nil {
			continue
		}
		if m.Source == "external" && !preferExternal && reg.Get(m.ID) != nil {
			continue
		}
		reg.Register(m)
	}
}
