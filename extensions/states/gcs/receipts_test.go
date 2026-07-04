package gcs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

// fakeGCS emulates the tiny slice of the JSON API the backend uses.
func fakeGCS(t *testing.T) (*httptest.Server, map[string][]byte) {
	t.Helper()
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/b/"):
			name := r.URL.Query().Get("name")
			body, _ := io.ReadAll(r.Body)
			objects[name] = body
			_ = json.NewEncoder(w).Encode(map[string]string{"name": name})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/o/"):
			name, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/base/b/bkt/o/"))
			data, ok := objects[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		case r.Method == http.MethodGet: // list
			prefix := r.URL.Query().Get("prefix")
			type item struct {
				Name string `json:"name"`
			}
			var items []item
			for name := range objects {
				if strings.HasPrefix(name, prefix) {
					items = append(items, item{Name: name})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case r.Method == http.MethodDelete:
			name, _ := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/base/b/bkt/o/"))
			if _, ok := objects[name]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(objects, name)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	return server, objects
}

func testBackend(t *testing.T, server *httptest.Server) *ReceiptBackend {
	t.Helper()
	t.Setenv("GCP_ACCESS_TOKEN", "test-token")
	client, err := New("bkt", "runfabric/dev")
	if err != nil {
		t.Fatal(err)
	}
	client.BaseURL = server.URL + "/base"
	client.UploadBaseURL = server.URL + "/upload"
	return NewReceiptBackend(client)
}

func TestGCSReceiptRoundTrip(t *testing.T) {
	server, objects := fakeGCS(t)
	defer server.Close()
	b := testBackend(t, server)

	receipt := &statetypes.Receipt{Service: "svc", Stage: "dev", Provider: "gcp-functions", UpdatedAt: "2026-07-04T00:00:00Z"}
	if err := b.Save(receipt); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := objects["runfabric/dev/receipts/dev.receipt.json"]; !ok {
		t.Fatalf("object not written at expected key, have %v", keysOf(objects))
	}

	loaded, err := b.Load("dev")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Service != "svc" || loaded.Stage != "dev" {
		t.Errorf("unexpected receipt: %+v", loaded)
	}

	releases, err := b.ListReleases()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(releases) != 1 || releases[0].Stage != "dev" || releases[0].UpdatedAt != "2026-07-04T00:00:00Z" {
		t.Errorf("unexpected releases: %+v", releases)
	}

	if err := b.Delete("dev"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(objects) != 0 {
		t.Errorf("object not deleted: %v", keysOf(objects))
	}
	// Deleting again (absent) is a tolerated no-op.
	if err := b.Delete("dev"); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
}

func TestGCSLoadMissingErrors(t *testing.T) {
	server, _ := fakeGCS(t)
	defer server.Close()
	b := testBackend(t, server)
	if _, err := b.Load("nope"); err == nil {
		t.Fatal("expected error for missing receipt")
	}
}

func TestGCSNewRequiresCredentialsAndBucket(t *testing.T) {
	t.Setenv("GCP_ACCESS_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	if _, err := New("bkt", "p"); err == nil || !strings.Contains(err.Error(), "GCP_ACCESS_TOKEN") {
		t.Fatalf("expected credentials error, got %v", err)
	}
	t.Setenv("GCP_ACCESS_TOKEN", "x")
	if _, err := New("", "p"); err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("expected bucket error, got %v", err)
	}
}

// TestGCSServiceAccountFlow proves the backend works with only a
// GOOGLE_APPLICATION_CREDENTIALS key: gcpauth exchanges it at a stub token
// endpoint and the minted token authenticates storage requests.
func TestGCSServiceAccountFlow(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	}))
	defer tokenSrv.Close()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
	keyJSON, _ := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": "svc@test.iam.gserviceaccount.com",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"token_uri":    tokenSrv.URL,
	})
	keyPath := filepath.Join(t.TempDir(), "sa.json")
	if err := os.WriteFile(keyPath, keyJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	server, _ := fakeGCS(t) // fake accepts "Bearer test-token"
	defer server.Close()

	t.Setenv("GCP_ACCESS_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath)
	client, err := New("bkt", "runfabric/dev")
	if err != nil {
		t.Fatalf("New with SA key: %v", err)
	}
	client.BaseURL = server.URL + "/base"
	client.UploadBaseURL = server.URL + "/upload"
	b := NewReceiptBackend(client)

	if err := b.Save(&statetypes.Receipt{Service: "svc", Stage: "dev", UpdatedAt: "2026-07-04T00:00:00Z"}); err != nil {
		t.Fatalf("save via SA-minted token: %v", err)
	}
	if _, err := b.Load("dev"); err != nil {
		t.Fatalf("load via SA-minted token: %v", err)
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
