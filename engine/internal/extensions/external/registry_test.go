package external

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/extensions/manifests"
)

func TestInstallFromRegistry_InstallsPluginWithGeneratedManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv(envHome, home)

	// Serve a fake binary and a resolve endpoint that references it.
	binBytes := []byte("fake-binary-bytes")
	sum := sha256.Sum256(binBytes)
	sumHex := hex.EncodeToString(sum[:])
	// Signature (ed25519 over the exact downloaded bytes).
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, binBytes)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	wantAuth := "Bearer test-token"
	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binBytes)
	})
	mux.HandleFunc("/v1/extensions/resolve", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != wantAuth {
			t.Fatalf("expected Authorization %q, got %q", wantAuth, got)
		}
		resp := map[string]any{
			"request": map[string]any{
				"id":   r.URL.Query().Get("id"),
				"core": r.URL.Query().Get("core"),
				"os":   r.URL.Query().Get("os"),
				"arch": r.URL.Query().Get("arch"),
			},
			"resolved": map[string]any{
				"id":          "provider-aws",
				"name":        "AWS Provider",
				"type":        "plugin",
				"pluginKind":  "provider",
				"version":     "1.0.0",
				"description": "test",
				"publisher": map[string]any{
					"verified": true,
				},
				"artifact": map[string]any{
					"type":      "binary",
					"format":    "executable",
					"url":       "", // filled below
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
			"meta": map[string]any{"registryVersion": "v1", "requestId": "t"},
		}
		// Use server base URL for bin.
		resp["resolved"].(map[string]any)["artifact"].(map[string]any)["url"] = serverURL(r) + "/bin"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := InstallFromRegistry(InstallFromRegistryOptions{
		RegistryURL: srv.URL,
		AuthToken:   "test-token",
		ID:          "provider-aws",
	}, "0.9.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Plugin == nil {
		t.Fatalf("expected plugin, got nil")
	}
	if res.Plugin.Kind != manifests.KindProvider {
		t.Fatalf("expected kind provider, got %s", res.Plugin.Kind)
	}
	dest := res.Plugin.Path
	if _, err := os.Stat(filepath.Join(dest, "plugin.yaml")); err != nil {
		t.Fatalf("expected plugin.yaml: %v", err)
	}
	if _, err := os.Stat(res.Plugin.Executable); err != nil {
		t.Fatalf("expected executable: %v", err)
	}

	receiptPath := filepath.Join(home, "registry-installs", "provider", "provider-aws", "1.0.0", "receipt.json")
	if _, err := os.Stat(receiptPath); err != nil {
		t.Fatalf("expected receipt.json: %v", err)
	}

	var receipt map[string]any
	b, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	if err := json.Unmarshal(b, &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v", err)
	}
	if receipt["requestId"] != "t" {
		t.Fatalf("expected receipt requestId=%q, got %v", "t", receipt["requestId"])
	}
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
