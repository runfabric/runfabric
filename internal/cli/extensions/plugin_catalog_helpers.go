package extensions

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/runfabric/runfabric/platform/extensions/application/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"github.com/runfabric/runfabric/platform/extensions/registry/resolution"
)

func discoverPluginCatalog(preferExternal bool, includeInvalid bool, pinned map[string]string) (*resolution.PluginCatalog, error) {
	return resolution.DiscoverPluginCatalog(external.DiscoverOptions{
		PreferExternal: preferExternal || external.PreferExternalFromEnv(),
		IncludeInvalid: includeInvalid,
		PinnedVersions: pinned,
	})
}

func writePrettyJSON(out io.Writer, payload any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func renderPluginManifestInfo(out io.Writer, m *manifests.PluginManifest) {
	fmt.Fprintf(out, "id:          %s\n", m.ID)
	fmt.Fprintf(out, "kind:        %s\n", m.Kind)
	fmt.Fprintf(out, "name:        %s\n", m.Name)
	fmt.Fprintf(out, "description: %s\n", m.Description)
	if m.Source != "" {
		fmt.Fprintf(out, "source:      %s\n", m.Source)
	}
	if m.Version != "" {
		fmt.Fprintf(out, "version:     %s\n", m.Version)
	}
	if m.Path != "" {
		fmt.Fprintf(out, "path:        %s\n", m.Path)
	}
	if m.Executable != "" {
		fmt.Fprintf(out, "executable:  %s\n", m.Executable)
	}
}
