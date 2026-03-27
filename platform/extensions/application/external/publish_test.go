package external

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPublishFlow_InitUploadFinalizeStatus(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "plugin.zip")
	artifactBytes := []byte("artifact-bytes-for-publish-test")
	if err := os.WriteFile(artifactPath, artifactBytes, 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	var mu sync.Mutex
	uploaded := false
	finalized := false
	publishID := "pub_test_1"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/extensions/publish/init", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer publish-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode init request: %v", err)
		}
		ext, _ := req["extension"].(map[string]any)
		if ext["id"] != "acme-test-provider" {
			t.Fatalf("unexpected extension id: %v", ext["id"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"publishId": publishID,
			"status":    "staged",
			"uploads": []map[string]any{
				{
					"key":    "artifact",
					"method": "PUT",
					"url":    serverURL(r) + "/upload/" + publishID + "/artifact",
				},
			},
		})
	})
	mux.HandleFunc("/upload/"+publishID+"/artifact", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT upload, got %s", r.Method)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upload body: %v", err)
		}
		if string(b) != string(artifactBytes) {
			t.Fatalf("uploaded bytes mismatch")
		}
		mu.Lock()
		uploaded = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v1/extensions/publish/finalize", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode finalize request: %v", err)
		}
		if req["publishId"] != publishID {
			t.Fatalf("unexpected publishId: %v", req["publishId"])
		}
		mu.Lock()
		defer mu.Unlock()
		if !uploaded {
			t.Fatalf("finalize called before upload")
		}
		finalized = true
		_ = json.NewEncoder(w).Encode(map[string]any{
			"publishId": publishID,
			"status":    "published",
		})
	})
	mux.HandleFunc("/v1/publish/"+publishID, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		status := "staged"
		if finalized {
			status = "published"
		}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"publishId": publishID,
			"status":    status,
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	pf, err := BuildPublishFileDescriptor("artifact", artifactPath)
	if err != nil {
		t.Fatalf("build descriptor: %v", err)
	}

	initRes, err := PublishInit(PublishInitOptions{
		RegistryURL: srv.URL,
		AuthToken:   "publish-token",
		ID:          "acme-test-provider",
		Version:     "1.0.0",
		Type:        "plugin",
		PluginKind:  "provider",
		Files:       []PublishFile{pf},
	})
	if err != nil {
		t.Fatalf("publish init: %v", err)
	}
	if initRes.PublishID != publishID || len(initRes.Uploads) != 1 {
		t.Fatalf("unexpected init response: %+v", initRes)
	}

	if err := UploadPublishFile(initRes.Uploads[0], artifactPath, 0); err != nil {
		t.Fatalf("publish upload: %v", err)
	}

	finRes, err := PublishFinalize(PublishFinalizeOptions{
		RegistryURL: srv.URL,
		AuthToken:   "publish-token",
		PublishID:   publishID,
	})
	if err != nil {
		t.Fatalf("publish finalize: %v", err)
	}
	if finRes.Status != "published" {
		t.Fatalf("unexpected finalize status: %q", finRes.Status)
	}

	statusRes, err := PublishStatus(PublishStatusOptions{
		RegistryURL: srv.URL,
		AuthToken:   "publish-token",
		PublishID:   publishID,
	})
	if err != nil {
		t.Fatalf("publish status: %v", err)
	}
	if statusRes.Status != "published" {
		t.Fatalf("unexpected publish status: %q", statusRes.Status)
	}
}
