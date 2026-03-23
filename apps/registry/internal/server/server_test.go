package server

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runfabric/runfabric/registry/internal/store"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{Store: st, AllowAnonymousRead: true, ArtifactSigningSecret: "test-secret"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv.Handler()
}

func newTestServerWithOptions(t *testing.T, opts Options) http.Handler {
	t.Helper()
	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	opts.Store = st
	if strings.TrimSpace(opts.ArtifactSigningSecret) == "" {
		opts.ArtifactSigningSecret = "test-secret"
	}
	srv, err := New(opts)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv.Handler()
}

func TestFrontend_StaticAndSPARoutes(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><html><body>registry-spa</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(webDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "assets", "app.abc123.js"), []byte("console.log('ok');"), 0o644); err != nil {
		t.Fatalf("write js asset: %v", err)
	}

	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		WebDir:             webDir,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "registry-spa") {
		t.Fatalf("root route should serve index: status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/docs/extension-development-guide", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "registry-spa") {
		t.Fatalf("spa route should serve index: status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/app.abc123.js", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "console.log") {
		t.Fatalf("asset route should serve static file: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("asset cache header mismatch: %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown api route expected 404 got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "registry-spa") || !strings.Contains(rr.Body.String(), "\"error\"") {
		t.Fatalf("unknown api route should return API envelope, body=%s", rr.Body.String())
	}
}

func TestUIConfigEndpoint(t *testing.T) {
	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		UIAuthURL:          "https://sso.example.com/login",
		UIDocsURL:          "https://docs.example.com/cli",
		OIDCIssuer:         "https://auth.example.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/ui/config", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ui config status=%d body=%s", rr.Code, rr.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode ui config: %v", err)
	}
	if got := payload["authLoginURL"]; got != "https://sso.example.com/login" {
		t.Fatalf("authLoginURL mismatch: %v", got)
	}
	if got := payload["cliDocsURL"]; got != "https://docs.example.com/cli" {
		t.Fatalf("cliDocsURL mismatch: %v", got)
	}
	if got := payload["oidcIssuer"]; got != "https://auth.example.com" {
		t.Fatalf("oidcIssuer mismatch: %v", got)
	}
}

func TestJWT_JWKSVerification(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes())
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "kid": "k1", "alg": "RS256", "n": n, "e": e},
			},
		})
	}))
	defer jwks.Close()

	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{
		Store:                 st,
		AllowAnonymousRead:    true,
		ArtifactSigningSecret: "test-secret",
		OIDCJWKSURL:           jwks.URL,
		OIDCIssuer:            "https://auth.test",
		OIDCAudience:          "registry",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := srv.Handler()

	token := testRS256JWT(t, priv, "k1", map[string]any{
		"sub":       "u1",
		"tenant_id": "tenant_runfabric",
		"roles":     []string{"admin"},
		"iss":       "https://auth.test",
		"aud":       "registry",
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"jwks-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("jwks jwt create package status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWT_JWKSDiscoveryFromIssuer(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes())

	mux := http.NewServeMux()
	var issuer string
	mux.HandleFunc("/realms/runfabric/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   issuer,
			"jwks_uri": issuer + "/protocol/openid-connect/certs",
		})
	})
	mux.HandleFunc("/realms/runfabric/protocol/openid-connect/certs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "kid": "k1", "alg": "RS256", "n": n, "e": e},
			},
		})
	})
	oidc := httptest.NewServer(mux)
	defer oidc.Close()
	issuer = oidc.URL + "/realms/runfabric"

	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{
		Store:                 st,
		AllowAnonymousRead:    true,
		ArtifactSigningSecret: "test-secret",
		OIDCIssuer:            issuer,
		OIDCAudience:          "registry",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	token := testRS256JWT(t, priv, "k1", map[string]any{
		"sub":       "u1",
		"tenant_id": "tenant_runfabric",
		"roles":     []string{"admin"},
		"iss":       issuer,
		"aud":       "registry",
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"discovery-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("oidc discovery create package status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWT_JWKSExplicitOverrideWins(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.PublicKey.E)).Bytes())
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "kid": "k1", "alg": "RS256", "n": n, "e": e},
			},
		})
	}))
	defer jwks.Close()

	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{
		Store:                 st,
		AllowAnonymousRead:    true,
		ArtifactSigningSecret: "test-secret",
		OIDCIssuer:            "https://issuer.example.invalid",
		OIDCAudience:          "registry",
		OIDCJWKSURL:           jwks.URL,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	token := testRS256JWT(t, priv, "k1", map[string]any{
		"sub":       "u1",
		"tenant_id": "tenant_runfabric",
		"roles":     []string{"admin"},
		"iss":       "https://issuer.example.invalid",
		"aud":       "registry",
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"jwks-override-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("jwks override create package status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWT_ES256JWKSVerification(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	x := base64.RawURLEncoding.EncodeToString(priv.PublicKey.X.Bytes())
	y := base64.RawURLEncoding.EncodeToString(priv.PublicKey.Y.Bytes())
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "EC", "kid": "ec1", "alg": "ES256", "crv": "P-256", "x": x, "y": y},
			},
		})
	}))
	defer jwks.Close()

	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{
		Store:                 st,
		AllowAnonymousRead:    true,
		ArtifactSigningSecret: "test-secret",
		OIDCJWKSURL:           jwks.URL,
		OIDCIssuer:            "https://auth.es256.test",
		OIDCAudience:          "registry",
		OIDCAllowedJWTAlgs:    "ES256",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	token := testES256JWT(t, priv, "ec1", map[string]any{
		"sub":       "u-es",
		"tenant_id": "tenant_runfabric",
		"roles":     []string{"admin"},
		"iss":       "https://auth.es256.test",
		"aud":       "registry",
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"es256-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("es256 jwt create package status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestS3PresignForArtifactURLs(t *testing.T) {
	tmp := t.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{
		Store:                 st,
		AllowAnonymousRead:    true,
		ArtifactSigningSecret: "test-secret",
		S3Bucket:              "registry-artifacts",
		S3Region:              "ap-southeast-1",
		S3AccessKeyID:         "AKIATESTKEY",
		S3SecretAccessKey:     "test-secret-key",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"s3-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/packages/acme/s3-demo/versions", bytes.NewReader([]byte(`{"version":"1.0.0"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish version status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/packages/acme/s3-demo/versions/1.0.0/download-url", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("download-url status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "X-Amz-Signature") {
		t.Fatalf("expected S3 presigned URL in response, body=%s", rr.Body.String())
	}
}

func TestResolveAndSearchEndpoints(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/extensions/resolve?id=provider-aws&core=0.9.0&os=darwin&arch=arm64", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("resolve status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/extensions/search?q=provider&type=plugin", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/extensions/list?type=plugin&page=1&pageSize=5", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestListEndpoint_RejectsSearchQueryParam(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/extensions/list?q=provider", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("list with q status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "q is not supported for list") {
		t.Fatalf("expected list contract error message, body=%s", rr.Body.String())
	}
}

func TestPublishFlowEndpoints(t *testing.T) {
	h := newTestServer(t)

	body := []byte("plugin-binary")
	sum := sha256.Sum256(body)
	initPayload := map[string]any{
		"extension": map[string]any{
			"id":         "provider-http",
			"version":    "1.0.0",
			"type":       "plugin",
			"pluginKind": "provider",
		},
		"files": []map[string]any{{
			"key":       "artifact",
			"name":      "plugin.bin",
			"sizeBytes": len(body),
			"checksum": map[string]any{
				"algorithm": "sha256",
				"value":     hex.EncodeToString(sum[:]),
			},
		}},
	}
	b, _ := json.Marshal(initPayload)
	req := httptest.NewRequest(http.MethodPost, "/v1/extensions/publish/init", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("publish init status=%d body=%s", rr.Code, rr.Body.String())
	}
	var initResp struct {
		PublishID string `json:"publishId"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("decode init response: %v", err)
	}
	if initResp.PublishID == "" {
		t.Fatal("missing publishId")
	}

	req = httptest.NewRequest(http.MethodPut, "/v1/uploads/"+initResp.PublishID+"/artifact", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", rr.Code, rr.Body.String())
	}

	finalizeBody, _ := json.Marshal(map[string]any{"publishId": initResp.PublishID})
	req = httptest.NewRequest(http.MethodPost, "/v1/extensions/publish/finalize", bytes.NewReader(finalizeBody))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("finalize status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/extensions/resolve?id=provider-http&core=0.9.0&os=darwin&arch=arm64", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("resolve published status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRateLimitPublishInit(t *testing.T) {
	h := newTestServer(t)
	payload := []byte(`{"extension":{"id":"x","version":"1.0.0","type":"addon"},"files":[{"key":"artifact","name":"a.tgz","sizeBytes":1,"checksum":{"algorithm":"sha256","value":"ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"}}]}`)
	lastCode := 0
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/extensions/publish/init", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer local-dev-token")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		lastCode = rr.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected final status 429, got %d", lastCode)
	}
}

func TestPackages_AnonymousReadAndAPIKeyWrite(t *testing.T) {
	h := newTestServer(t)

	createBody := []byte(`{"namespace":"acme","name":"demo","visibility":"public"}`)
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "ApiKey rk_local_dev")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	versionBody := []byte(`{"version":"1.0.0","manifest_json":{"runtime":"nodejs"}}`)
	req = httptest.NewRequest(http.MethodPost, "/packages/acme/demo/versions", bytes.NewReader(versionBody))
	req.Header.Set("Authorization", "ApiKey rk_local_dev")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish version status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/packages", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("anonymous list status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"demo"`) {
		t.Fatalf("expected package in anonymous list, body=%s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/packages/acme/demo", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("anonymous detail status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPackages_TenantIsolationWithJWT(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"private","visibility":"tenant"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create private package status=%d body=%s", rr.Code, rr.Body.String())
	}

	otherJWT := testJWT(map[string]any{
		"sub":       "other-user",
		"tenant_id": "tenant_other",
		"roles":     []string{"admin"},
	})
	req = httptest.NewRequest(http.MethodDelete, "/packages/acme/private", nil)
	req.Header.Set("Authorization", "Bearer "+otherJWT)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("expected tenant isolation failure, got status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPackages_ArtifactSignedURLs(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"artifact-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/packages/acme/artifact-demo/versions", bytes.NewReader([]byte(`{"version":"1.0.0"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish version status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/packages/acme/artifact-demo/versions/1.0.0/upload-url", nil)
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload-url status=%d body=%s", rr.Code, rr.Body.String())
	}
	var uploadResp struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("decode upload url response: %v", err)
	}
	u, err := url.Parse(uploadResp.URL)
	if err != nil {
		t.Fatalf("parse upload url: %v", err)
	}

	payload := []byte("artifact-bytes")
	req = httptest.NewRequest(http.MethodPut, u.RequestURI(), bytes.NewReader(payload))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("artifact PUT status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/packages/acme/artifact-demo/versions/1.0.0/download-url", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("download-url status=%d body=%s", rr.Code, rr.Body.String())
	}
	var downloadResp struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &downloadResp); err != nil {
		t.Fatalf("decode download url response: %v", err)
	}
	du, err := url.Parse(downloadResp.URL)
	if err != nil {
		t.Fatalf("parse download url: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, du.RequestURI(), nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("artifact GET status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != string(payload) {
		t.Fatalf("artifact content mismatch got=%q want=%q", rr.Body.String(), string(payload))
	}
}

func TestPackages_RedisFallbackWhenUnavailable(t *testing.T) {
	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		RedisAddr:          "127.0.0.1:1",
	})

	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"cache-fallback","visibility":"public"}`)))
	req.Header.Set("Authorization", "ApiKey rk_local_dev")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	for i := 0; i < 3; i++ {
		req = httptest.NewRequest(http.MethodGet, "/packages", nil)
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("list packages status=%d body=%s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), `"cache-fallback"`) {
			t.Fatalf("expected package in response, body=%s", rr.Body.String())
		}
	}
}

func TestJWT_TenantClaimMustUseCanonicalTenantID(t *testing.T) {
	h := newTestServer(t)
	token := testJWT(map[string]any{
		"sub":   "u1",
		"tid":   "tenant_runfabric",
		"roles": []string{"admin"},
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"tenant-claim-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-canonical tenant claim, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWT_ClaimMappingSupportsAuth0NamespacedClaims(t *testing.T) {
	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		OIDCSubjectClaim:   "sub",
		OIDCTenantClaim:    "https://runfabric.cloud/tenant_id",
		OIDCRolesClaim:     "https://runfabric.cloud/roles",
		OIDCRoleModes:      "roles",
	})
	token := testJWT(map[string]any{
		"sub":                               "auth0-user",
		"https://runfabric.cloud/tenant_id": "tenant_runfabric",
		"https://runfabric.cloud/roles":     []string{"admin"},
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"auth0-claims-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected auth0 namespaced claim mapping to pass, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWT_RoleExtractionPrecedence_KeycloakResourceAccess(t *testing.T) {
	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		OIDCRoleModes:      "resource_access.<client>.roles,realm_access.roles,scope",
		OIDCRoleClientID:   "registry-cli",
	})
	token := testJWT(map[string]any{
		"sub":       "kc-user",
		"tenant_id": "tenant_runfabric",
		"realm_access": map[string]any{
			"roles": []string{"reader"},
		},
		"resource_access": map[string]any{
			"registry-cli": map[string]any{
				"roles": []string{"admin"},
			},
		},
		"scope": "openid profile registry:read",
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"keycloak-role-mode-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected keycloak resource_access role mode to grant admin, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWT_AudienceModes(t *testing.T) {
	handlerIncludes := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		OIDCAudience:       "registry",
		OIDCAudienceMode:   "includes",
	})
	tokenIncludes := testJWT(map[string]any{
		"sub":       "aud-user",
		"tenant_id": "tenant_runfabric",
		"roles":     []string{"admin"},
		"aud":       "urn:runfabric:registry service",
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"aud-includes-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+tokenIncludes)
	rr := httptest.NewRecorder()
	handlerIncludes.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("audience includes mode should allow request, got=%d body=%s", rr.Code, rr.Body.String())
	}

	handlerSkip := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		OIDCAudience:       "registry",
		OIDCAudienceMode:   "skip",
	})
	tokenSkip := testJWT(map[string]any{
		"sub":       "aud-skip-user",
		"tenant_id": "tenant_runfabric",
		"roles":     []string{"admin"},
	})
	req = httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"aud-skip-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+tokenSkip)
	rr = httptest.NewRecorder()
	handlerSkip.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("audience skip mode should allow request without aud, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPackages_CustomArtifactKeyIsCanonicalized(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"canonical-key-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/packages/acme/canonical-key-demo/versions", bytes.NewReader([]byte(`{"version":"1.0.0","artifact_key":"tenants/tenant_other/packages/x/y/z/artifact.tar.gz"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish version status=%d body=%s", rr.Code, rr.Body.String())
	}
	var rec struct {
		ArtifactKey string `json:"artifact_key"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	want := "tenants/tenant_runfabric/packages/acme/canonical-key-demo/1.0.0/artifact.tar.gz"
	if rec.ArtifactKey != want {
		t.Fatalf("artifact_key=%q want=%q", rec.ArtifactKey, want)
	}
}

func TestPackages_CrossTenantUploadURLForbidden(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"cross-tenant-upload","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/packages/acme/cross-tenant-upload/versions", bytes.NewReader([]byte(`{"version":"1.0.0"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish version status=%d body=%s", rr.Code, rr.Body.String())
	}

	otherJWT := testJWT(map[string]any{
		"sub":       "other-user",
		"tenant_id": "tenant_other",
		"roles":     []string{"admin"},
	})
	req = httptest.NewRequest(http.MethodPost, "/packages/acme/cross-tenant-upload/versions/1.0.0/upload-url", nil)
	req.Header.Set("Authorization", "Bearer "+otherJWT)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant upload-url, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestTenantAwareRBACBindings(t *testing.T) {
	policy := strings.TrimSpace(`
p, admin, package, package:publish, allow
p, reader, package, package:read, allow
g, shared-user, admin, tenant_a
g, shared-user, reader, tenant_b
`)
	tmp := t.TempDir()
	policyPath := filepath.Join(tmp, "policy.csv")
	if err := os.WriteFile(policyPath, []byte(policy+"\n"), 0o644); err != nil {
		t.Fatalf("write policy file: %v", err)
	}
	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		CasbinPolicyPath:   policyPath,
	})

	tokenTenantA := testJWT(map[string]any{
		"sub":       "shared-user",
		"tenant_id": "tenant_a",
		"roles":     []string{"admin"},
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"rbac-tenant-a","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+tokenTenantA)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("tenant_a publish should be allowed, got=%d body=%s", rr.Code, rr.Body.String())
	}

	tokenTenantB := testJWT(map[string]any{
		"sub":       "shared-user",
		"tenant_id": "tenant_b",
		"roles":     []string{"admin"},
	})
	req = httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"rbac-tenant-b","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+tokenTenantB)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("tenant_b publish should be forbidden by tenant binding, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestTenantAwareRBACBindings_NoFallbackForBoundSubjectInOtherTenant(t *testing.T) {
	policy := strings.TrimSpace(`
p, admin, package, package:publish, allow
g, shared-user, admin, tenant_a
`)
	tmp := t.TempDir()
	policyPath := filepath.Join(tmp, "policy.csv")
	if err := os.WriteFile(policyPath, []byte(policy+"\n"), 0o644); err != nil {
		t.Fatalf("write policy file: %v", err)
	}
	h := newTestServerWithOptions(t, Options{
		AllowAnonymousRead: true,
		CasbinPolicyPath:   policyPath,
	})

	tokenTenantA := testJWT(map[string]any{
		"sub":       "shared-user",
		"tenant_id": "tenant_a",
		"roles":     []string{"admin"},
	})
	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"rbac-bound-a","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+tokenTenantA)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("tenant_a publish should be allowed, got=%d body=%s", rr.Code, rr.Body.String())
	}

	tokenTenantC := testJWT(map[string]any{
		"sub":       "shared-user",
		"tenant_id": "tenant_c",
		"roles":     []string{"admin"},
	})
	req = httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"rbac-bound-c","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+tokenTenantC)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("tenant_c publish should be forbidden without tenant binding, got=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuditIncludesTenantAndActorIDs(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/packages", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("anonymous list status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"audit-fields-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/audit?limit=50", nil)
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(resp.Events) == 0 {
		t.Fatal("expected audit events")
	}
	for _, ev := range resp.Events {
		actorID, _ := ev["actor_id"].(string)
		tenantID, _ := ev["tenant_id"].(string)
		if strings.TrimSpace(actorID) == "" || strings.TrimSpace(tenantID) == "" {
			t.Fatalf("audit event missing actor_id/tenant_id: %#v", ev)
		}
		if tenantID != "tenant_runfabric" {
			t.Fatalf("expected tenant-scoped audit event, got tenant_id=%q event=%#v", tenantID, ev)
		}
	}
}

func TestAuditEndpoint_IsTenantScoped(t *testing.T) {
	h := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"tenant-audit-a","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("tenant_runfabric create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	otherJWT := testJWT(map[string]any{
		"sub":       "other-user",
		"tenant_id": "tenant_other",
		"roles":     []string{"admin"},
	})
	req = httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"tenant-audit-b","visibility":"public"}`)))
	req.Header.Set("Authorization", "Bearer "+otherJWT)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("tenant_other create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/audit?limit=100", nil)
	req.Header.Set("Authorization", "Bearer local-dev-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("tenant_runfabric audit status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(resp.Events) == 0 {
		t.Fatal("expected tenant-scoped audit events")
	}
	for _, ev := range resp.Events {
		tenantID, _ := ev["tenant_id"].(string)
		if tenantID != "tenant_runfabric" {
			t.Fatalf("expected tenant-scoped audit events, found tenant_id=%q event=%#v", tenantID, ev)
		}
	}
}

func BenchmarkPackagesListCachedRead(b *testing.B) {
	tmp := b.TempDir()
	st, err := store.Open(store.OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	srv, err := New(Options{Store: st, AllowAnonymousRead: true, ArtifactSigningSecret: "bench-secret"})
	if err != nil {
		b.Fatalf("new server: %v", err)
	}
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/packages", bytes.NewReader([]byte(`{"namespace":"acme","name":"bench-demo","visibility":"public"}`)))
	req.Header.Set("Authorization", "ApiKey rk_local_dev")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		b.Fatalf("create package status=%d body=%s", rr.Code, rr.Body.String())
	}

	warmReq := httptest.NewRequest(http.MethodGet, "/packages", nil)
	warmResp := httptest.NewRecorder()
	h.ServeHTTP(warmResp, warmReq)
	if warmResp.Code != http.StatusOK {
		b.Fatalf("warmup status=%d body=%s", warmResp.Code, warmResp.Body.String())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/packages", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("list packages status=%d body=%s", rr.Code, rr.Body.String())
		}
	}
}

func testJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".x"
}

func testRS256JWT(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerBytes, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid})
	payloadBytes, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signing := header + "." + payload
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func testES256JWT(t *testing.T, key *ecdsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerBytes, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "JWT", "kid": kid})
	payloadBytes, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signing := header + "." + payload
	sum := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		t.Fatalf("sign es256 jwt: %v", err)
	}
	size := (key.Params().BitSize + 7) / 8
	rb := r.FillBytes(make([]byte, size))
	sb := s.FillBytes(make([]byte, size))
	sig := append(rb, sb...)
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}
