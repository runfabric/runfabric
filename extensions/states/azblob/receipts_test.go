package azblob

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	statetypes "github.com/runfabric/runfabric/extensions/types"
)

// fakeBlob emulates the slice of the Blob REST API the backend uses.
// It checks that requests are SharedKey-signed (header shape, not the HMAC).
func fakeBlob(t *testing.T) (*httptest.Server, map[string][]byte) {
	t.Helper()
	blobs := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "SharedKey testacct:") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("x-ms-version") == "" || r.Header.Get("x-ms-date") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/ctr/")
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("comp") == "list":
			prefix := r.URL.Query().Get("prefix")
			var items strings.Builder
			for blobName := range blobs {
				if strings.HasPrefix(blobName, prefix) {
					items.WriteString("<Blob><Name>" + blobName + "</Name></Blob>")
				}
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><EnumerationResults><Blobs>%s</Blobs><NextMarker/></EnumerationResults>`, items.String())
		case r.Method == http.MethodPut:
			if r.Header.Get("x-ms-blob-type") != "BlockBlob" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(r.Body)
			blobs[name] = body
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet:
			data, ok := blobs[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		case r.Method == http.MethodDelete:
			if _, ok := blobs[name]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(blobs, name)
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	return server, blobs
}

func testBackend(t *testing.T, server *httptest.Server) *ReceiptBackend {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("test-key"))
	t.Setenv("AZURE_STORAGE_CONNECTION_STRING",
		"BlobEndpoint="+server.URL+";AccountName=testacct;AccountKey="+key)
	client, err := New("ctr", "runfabric/dev")
	if err != nil {
		t.Fatal(err)
	}
	return NewReceiptBackend(client)
}

func TestAzblobReceiptRoundTrip(t *testing.T) {
	server, blobs := fakeBlob(t)
	defer server.Close()
	b := testBackend(t, server)

	receipt := &statetypes.Receipt{Service: "svc", Stage: "dev", Provider: "azure-functions", UpdatedAt: "2026-07-04T00:00:00Z"}
	if err := b.Save(receipt); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := blobs["runfabric/dev/receipts/dev.receipt.json"]; !ok {
		t.Fatalf("blob not written at expected key, have %v", blobs)
	}

	loaded, err := b.Load("dev")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Service != "svc" || loaded.Provider != "azure-functions" {
		t.Errorf("unexpected receipt: %+v", loaded)
	}

	releases, err := b.ListReleases()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(releases) != 1 || releases[0].Stage != "dev" {
		t.Errorf("unexpected releases: %+v", releases)
	}

	if err := b.Delete("dev"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := b.Delete("dev"); err != nil {
		t.Fatalf("delete absent should be tolerated: %v", err)
	}
}

func TestAzblobNewCredentialSources(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("k"))

	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", "")
	t.Setenv("AZURE_STORAGE_ACCOUNT", "acct2")
	t.Setenv("AZURE_STORAGE_KEY", key)
	c, err := New("ctr", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Endpoint != "https://acct2.blob.core.windows.net" {
		t.Errorf("endpoint = %q", c.Endpoint)
	}

	t.Setenv("AZURE_STORAGE_ACCOUNT", "")
	t.Setenv("AZURE_STORAGE_KEY", "")
	if _, err := New("ctr", ""); err == nil || !strings.Contains(err.Error(), "AZURE_STORAGE_CONNECTION_STRING") {
		t.Fatalf("expected credentials error, got %v", err)
	}

	t.Setenv("AZURE_STORAGE_CONNECTION_STRING", "DefaultEndpointsProtocol=https;AccountName=a;AccountKey=not-base64!!;EndpointSuffix=core.windows.net")
	if _, err := New("ctr", ""); err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("expected base64 error, got %v", err)
	}
}

func TestAzblobSharedKeyStringToSign(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("secret"))
	c := &Client{Endpoint: "https://acct.blob.core.windows.net", Account: "acct", Key: key, Container: "ctr"}
	req, _ := http.NewRequest(http.MethodGet, "https://acct.blob.core.windows.net/ctr?restype=container&comp=list&prefix=receipts/", nil)
	req.Header.Set("x-ms-date", "Thu, 04 Jul 2026 00:00:00 GMT")
	req.Header.Set("x-ms-version", apiVersion)
	auth, err := c.sharedKeyAuth(req, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(auth, "SharedKey acct:") {
		t.Errorf("auth header shape wrong: %q", auth)
	}
	// Same inputs must sign deterministically.
	auth2, _ := c.sharedKeyAuth(req, 0)
	if auth != auth2 {
		t.Error("signature not deterministic")
	}
}
