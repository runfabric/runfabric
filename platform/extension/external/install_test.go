package external

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	manifests "github.com/runfabric/runfabric/platform/extensions/manifest"
	"gopkg.in/yaml.v3"
)

func TestInstall_WithoutSource_ResolvesFromRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envHome, home)

	binBytes := []byte("registry-bin")
	sum := sha256.Sum256(binBytes)
	sumHex := hex.EncodeToString(sum[:])
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sigB64 := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, binBytes))

	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binBytes)
	})
	mux.HandleFunc("/v1/extensions/resolve", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"request": map[string]any{
				"id":   "provider-aws",
				"core": r.URL.Query().Get("core"),
				"os":   r.URL.Query().Get("os"),
				"arch": r.URL.Query().Get("arch"),
			},
			"resolved": map[string]any{
				"id":         "provider-aws",
				"name":       "AWS Provider",
				"type":       "plugin",
				"pluginKind": "providers",
				"version":    "1.0.0",
				"publisher": map[string]any{
					"verified": true,
				},
				"artifact": map[string]any{
					"type":      "binary",
					"format":    "executable",
					"url":       serverURL(r) + "/bin",
					"sizeBytes": len(binBytes),
					"checksum": map[string]any{
						"algorithm": "sha256",
						"value":     sumHex,
					},
					"signature": map[string]any{
						"algorithm":   "ed25519",
						"value":       sigB64,
						"publicKeyId": "local-dev",
					},
				},
			},
			"meta": map[string]any{"requestId": "resolve-install"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := Install(InstallOptions{
		ID:          "provider-aws",
		Version:     "1.0.0",
		RegistryURL: srv.URL,
		CoreVersion: "0.9.0",
	})
	if err != nil {
		t.Fatalf("install via registry resolution failed: %v", err)
	}
	if res == nil || res.Plugin == nil {
		t.Fatalf("expected installed plugin")
	}
	if res.Plugin.Kind != manifests.KindProvider {
		t.Fatalf("expected provider kind, got %q", res.Plugin.Kind)
	}
}

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
