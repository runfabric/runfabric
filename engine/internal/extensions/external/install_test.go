package external

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
	"gopkg.in/yaml.v3"
)

func TestInstallAndUninstall_FromLocalTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin executable test not supported on windows")
	}

	home := t.TempDir()
	t.Setenv(envHome, home)

	buildDir := t.TempDir()
	exe := filepath.Join(buildDir, "stubplugin")
	cmd := exec.Command("go", "build", "-o", exe, "./testdata/stubplugin")
	cmd.Dir = "." // engine/internal/extensions/external
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build stubplugin: %v\n%s", err, string(out))
	}

	pluginRoot := t.TempDir()
	pm := pluginYAML{
		APIVersion:  "runfabric.io/v1alpha1",
		Kind:        "provider",
		ID:          "stub",
		Name:        "Stub Provider",
		Description: "stub",
		Version:     "0.1.0",
		Executable:  "stubplugin",
	}
	pm.Permissions.FS = true
	yml, err := yaml.Marshal(pm)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.yaml"), yml, 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(exe); err == nil {
		if err := os.WriteFile(filepath.Join(pluginRoot, "stubplugin"), b, 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "stub-0.1.0.tar.gz")
	if err := writeTarGz(archive, pluginRoot); err != nil {
		t.Fatal(err)
	}

	res, err := Install(InstallOptions{
		ID:     "stub",
		Kind:   manifests.KindProvider,
		Source: archive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Plugin == nil || res.Plugin.Path == "" {
		t.Fatalf("expected plugin path, got %#v", res)
	}

	disc, err := Discover(DiscoverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range disc.Plugins {
		if p.ID == "stub" && p.Kind == manifests.KindProvider {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected discovered plugin 'stub'")
	}

	if err := Uninstall(UninstallOptions{ID: "stub", Kind: manifests.KindProvider}); err != nil {
		t.Fatal(err)
	}
	disc2, err := Discover(DiscoverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range disc2.Plugins {
		if p.ID == "stub" {
			t.Fatalf("expected stub removed, still discovered")
		}
	}
}

func writeTarGz(path string, dir string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		h := &tar.Header{
			Name: rel,
			Mode: int64(info.Mode()),
			Size: info.Size(),
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = tw.Write(b)
		return err
	})
}
