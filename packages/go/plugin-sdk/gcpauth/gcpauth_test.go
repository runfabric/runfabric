package gcpauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resetState() {
	mu.Lock()
	mintedToken = ""
	mintedExp = time.Time{}
	nowFunc = time.Now
	mu.Unlock()
}

// writeKeyFile generates an RSA service-account key file pointing token_uri at the stub.
func writeKeyFile(t *testing.T, tokenURI string) (string, *rsa.PrivateKey) {
	t.Helper()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	key := map[string]string{
		"type":         "service_account",
		"client_email": "svc@test.iam.gserviceaccount.com",
		"private_key":  string(pemKey),
		"token_uri":    tokenURI,
	}
	data, _ := json.Marshal(key)
	path := filepath.Join(t.TempDir(), "sa.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, rsaKey
}

// tokenStub validates the JWT-bearer grant shape and counts exchanges.
func tokenStub(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if g := r.Form.Get("grant_type"); g != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type = %q", g)
		}
		assertion := r.Form.Get("assertion")
		parts := strings.Split(assertion, ".")
		if len(parts) != 3 {
			t.Errorf("assertion is not a JWT: %q", assertion)
		} else {
			claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				t.Errorf("decode claims: %v", err)
			}
			var claims map[string]any
			_ = json.Unmarshal(claimsJSON, &claims)
			if claims["iss"] != "svc@test.iam.gserviceaccount.com" || claims["scope"] != Scope {
				t.Errorf("unexpected claims: %v", claims)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "minted-token", "expires_in": 3600})
	}))
}

func TestEnsureAccessToken_UserTokenWins(t *testing.T) {
	resetState()
	t.Setenv("GCP_ACCESS_TOKEN", "user-token")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/does/not/exist.json")
	if err := EnsureAccessToken(context.Background()); err != nil {
		t.Fatalf("user token must be a no-op: %v", err)
	}
	if os.Getenv("GCP_ACCESS_TOKEN") != "user-token" {
		t.Error("user token was overwritten")
	}
}

func TestEnsureAccessToken_MintsAndCaches(t *testing.T) {
	resetState()
	calls := 0
	stub := tokenStub(t, &calls)
	defer stub.Close()
	keyPath, _ := writeKeyFile(t, stub.URL)
	t.Setenv("GCP_ACCESS_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath)

	if err := EnsureAccessToken(context.Background()); err != nil {
		t.Fatalf("mint: %v", err)
	}
	if os.Getenv("GCP_ACCESS_TOKEN") != "minted-token" {
		t.Errorf("env not exported: %q", os.Getenv("GCP_ACCESS_TOKEN"))
	}
	// Fresh token → cached, no second exchange.
	if err := EnsureAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected 1 exchange, got %d", calls)
	}

	// Near expiry → re-mints our own token.
	mu.Lock()
	nowFunc = func() time.Time { return mintedExp.Add(-time.Minute) }
	mu.Unlock()
	if err := EnsureAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected refresh exchange, got %d calls", calls)
	}
}

func TestEnsureAccessToken_NeitherSourceErrors(t *testing.T) {
	resetState()
	t.Setenv("GCP_ACCESS_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	err := EnsureAccessToken(context.Background())
	if err == nil || !strings.Contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Fatalf("expected guidance error, got %v", err)
	}
}

func TestEnsureAccessToken_BadKeyFileErrors(t *testing.T) {
	resetState()
	path := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(path, []byte("{"), 0o600)
	t.Setenv("GCP_ACCESS_TOKEN", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	if err := EnsureAccessToken(context.Background()); err == nil {
		t.Fatal("expected error for malformed key file")
	}
}
