package resolve

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

type apiError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Hint      string         `json:"hint,omitempty"`
	DocsURL   string         `json:"docsUrl,omitempty"`
	RequestID string         `json:"requestId"`
}

// NewHandler returns a v1 resolve handler suitable for local development.
// This is intentionally minimal: it validates required query params and returns a deterministic
// response for a small hardcoded catalog.
func NewHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := requestIDFromRequest(r)
		if r.Method != http.MethodGet {
			writeAPIError(w, http.StatusMethodNotAllowed, apiError{
				Code:      "INVALID_REQUEST",
				Message:   "Method not allowed",
				Details:   map[string]any{"method": r.Method},
				RequestID: reqID,
			})
			return
		}

		q := r.URL.Query()
		id := strings.TrimSpace(q.Get("id"))
		core := strings.TrimSpace(q.Get("core"))
		os := strings.TrimSpace(q.Get("os"))
		arch := strings.TrimSpace(q.Get("arch"))
		version := strings.TrimSpace(q.Get("version"))

		if id == "" || core == "" || os == "" || arch == "" {
			missing := []string{}
			if id == "" {
				missing = append(missing, "id")
			}
			if core == "" {
				missing = append(missing, "core")
			}
			if os == "" {
				missing = append(missing, "os")
			}
			if arch == "" {
				missing = append(missing, "arch")
			}
			writeAPIError(w, http.StatusBadRequest, apiError{
				Code:      "INVALID_REQUEST",
				Message:   "Missing required query parameters",
				Details:   map[string]any{"missing": missing},
				Hint:      "Provide id, core, os, and arch query parameters.",
				DocsURL:   "https://runfabric.cloud/docs/extensions/registry#resolve",
				RequestID: reqID,
			})
			return
		}

		resolved, ok := catalogResolve(id, os, arch, version)
		if !ok {
			writeAPIError(w, http.StatusNotFound, apiError{
				Code:      "EXTENSION_NOT_FOUND",
				Message:   fmt.Sprintf("Extension %q was not found", id),
				Details:   map[string]any{"id": id},
				Hint:      "Check the extension ID or search available extensions.",
				DocsURL:   "https://runfabric.cloud/docs/extensions/search",
				RequestID: reqID,
			})
			return
		}

		out := map[string]any{
			"request": map[string]any{
				"id":   id,
				"core": core,
				"os":   os,
				"arch": arch,
			},
			"resolved": resolved,
			"meta": map[string]any{
				"resolvedAt":      time.Now().UTC().Format(time.RFC3339),
				"registryVersion": "v1",
				"requestId":       reqID,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	})
}

func writeAPIError(w http.ResponseWriter, status int, err apiError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": err})
}

func requestIDFromRequest(r *http.Request) string {
	// Allow upstream/proxy to inject a request id.
	if v := strings.TrimSpace(r.Header.Get("X-Request-Id")); v != "" {
		return v
	}
	// Local dev id.
	return fmt.Sprintf("req_local_%s_%d", runtime.GOOS, time.Now().UnixNano())
}

func catalogResolve(id, os, arch, version string) (map[string]any, bool) {
	// NOTE: These are illustrative only. Real registry will store artifacts + checksums/signatures.
	switch id {
	case "sentry":
		v := "1.2.0"
		if version != "" {
			v = version
		}
		// Create a deterministic-but-fake checksum from id+version for local testing.
		sum := sha256.Sum256([]byte(id + "@" + v))
		return map[string]any{
			"id":      "sentry",
			"name":    "Sentry",
			"type":    "addon",
			"version": v,
			"publisher": map[string]any{
				"name":     "runfabric",
				"verified": true,
				"trust":    "official",
			},
			"description": "Error tracking and performance monitoring for serverless functions",
			"compatibility": map[string]any{
				"core": ">=0.8.0",
				"runtimes": []string{
					"node",
				},
				"providers": []string{
					"aws",
					"cloudflare",
					"vercel",
				},
			},
			"permissions": []string{
				"env:write",
				"fs:build-write",
				"handler:wrap",
				"network:outbound",
			},
			"artifact": map[string]any{
				"type":      "addon",
				"format":    "tgz",
				"url":       fmt.Sprintf("https://cdn.runfabric.cloud/extensions/addons/sentry/%s/addon.tgz", v),
				"sizeBytes": 84231,
				"checksum": map[string]any{
					"algorithm": "sha256",
					"value":     hex.EncodeToString(sum[:]),
				},
			},
			"manifest": map[string]any{
				"url":       fmt.Sprintf("https://cdn.runfabric.cloud/extensions/addons/sentry/%s/manifest.json", v),
				"schemaUrl": fmt.Sprintf("https://cdn.runfabric.cloud/extensions/addons/sentry/%s/config.schema.json", v),
			},
			"integrity": map[string]any{
				"sbomUrl":       fmt.Sprintf("https://cdn.runfabric.cloud/extensions/addons/sentry/%s/sbom.spdx.json", v),
				"provenanceUrl": fmt.Sprintf("https://cdn.runfabric.cloud/extensions/addons/sentry/%s/provenance.intoto.jsonl", v),
			},
			"install": map[string]any{
				"path":        fmt.Sprintf("addons/sentry/%s", v),
				"postInstall": []string{},
			},
		}, true

	case "provider-aws":
		if os == "" || arch == "" {
			return nil, false
		}
		v := "1.0.0"
		if version != "" {
			v = version
		}
		sum := sha256.Sum256([]byte(id + "@" + v + ":" + os + "-" + arch))
		return map[string]any{
			"id":         "provider-aws",
			"name":       "AWS Provider",
			"type":       "plugin",
			"pluginKind": "provider",
			"version":    v,
			"publisher": map[string]any{
				"name":     "runfabric",
				"verified": true,
				"trust":    "official",
			},
			"compatibility": map[string]any{
				"core": ">=0.9.0",
			},
			"capabilities": []string{
				"validateConfig",
				"plan",
				"deploy",
				"remove",
				"invoke",
				"logs",
				"doctor",
			},
			"artifact": map[string]any{
				"type":      "binary",
				"format":    "executable",
				"url":       fmt.Sprintf("https://cdn.runfabric.cloud/extensions/plugins/providers/aws/%s/%s-%s/runfabric-plugin-provider-aws", v, os, arch),
				"sizeBytes": 18429312,
				"checksum": map[string]any{
					"algorithm": "sha256",
					"value":     hex.EncodeToString(sum[:]),
				},
			},
			"manifest": map[string]any{
				"url": fmt.Sprintf("https://cdn.runfabric.cloud/extensions/plugins/providers/aws/%s/manifest.json", v),
			},
			"integrity": map[string]any{
				"sbomUrl":       fmt.Sprintf("https://cdn.runfabric.cloud/extensions/plugins/providers/aws/%s/sbom.spdx.json", v),
				"provenanceUrl": fmt.Sprintf("https://cdn.runfabric.cloud/extensions/plugins/providers/aws/%s/provenance.intoto.jsonl", v),
			},
			"install": map[string]any{
				"path":   fmt.Sprintf("plugins/providers/aws/%s/%s-%s", v, os, arch),
				"binary": "runfabric-plugin-provider-aws",
			},
		}, true
	}
	return nil, false
}
