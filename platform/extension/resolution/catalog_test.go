package resolution

import (
	"testing"

	"github.com/runfabric/runfabric/platform/extension/external"
	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
)

func TestDiscoverPluginCatalog_BuiltinWinsUnlessPreferExternal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalProvider(t, home, "vercel", "1.2.3")

	catalog, err := DiscoverPluginCatalog(externalDiscover(false, "", ""))
	if err != nil {
		t.Fatalf("discover catalog: %v", err)
	}
	if m := catalog.Registry.Get("vercel"); m == nil || m.Source == "external" {
		t.Fatalf("expected builtin vercel to win by default, got %+v", m)
	}

	catalog, err = DiscoverPluginCatalog(externalDiscover(true, "", ""))
	if err != nil {
		t.Fatalf("discover catalog prefer external: %v", err)
	}
	if m := catalog.Registry.Get("vercel"); m == nil || m.Source != "external" {
		t.Fatalf("expected external vercel when preferExternal=true, got %+v", m)
	}
}

func TestHasInstalledExternalPlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RUNFABRIC_HOME", home)
	writeExternalProvider(t, home, "my-provider", "0.1.0")

	ok, err := HasInstalledExternalPlugin(manifests.KindProvider, "my-provider", "0.1.0")
	if err != nil {
		t.Fatalf("has installed external plugin: %v", err)
	}
	if !ok {
		t.Fatal("expected plugin to be discovered")
	}
}

func externalDiscover(prefer bool, id, version string) external.DiscoverOptions {
	opts := external.DiscoverOptions{PreferExternal: prefer}
	if id != "" && version != "" {
		opts.PinnedVersions = map[string]string{id: version}
	}
	return opts
}
