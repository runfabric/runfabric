// Package gcpauth resolves a GCP access token for provider/state extensions.
//
// Resolution order:
//  1. GCP_ACCESS_TOKEN already set (user/gcloud/daemon header) — used as-is.
//  2. GOOGLE_APPLICATION_CREDENTIALS — a service-account key file, exchanged
//     for an OAuth2 access token via the JWT-bearer grant and exported as
//     GCP_ACCESS_TOKEN so every Env-based call site keeps working. Minted
//     tokens are cached and re-minted shortly before expiry, which gives
//     long-running daemons stable credentials instead of hourly tokens.
package gcpauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Scope covers Cloud Functions, Cloud Storage, Logging and Monitoring.
const Scope = "https://www.googleapis.com/auth/cloud-platform"

const defaultTokenURI = "https://oauth2.googleapis.com/token"

// refreshMargin re-mints tokens this long before they expire.
const refreshMargin = 5 * time.Minute

var (
	mu          sync.Mutex
	mintedToken string
	mintedExp   time.Time
	// nowFunc and HTTPClient are injectable for tests.
	nowFunc                 = time.Now
	HTTPClient *http.Client = &http.Client{Timeout: 30 * time.Second}
)

type serviceAccountKey struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// EnsureAccessToken makes sure GCP_ACCESS_TOKEN carries a usable token.
// No-op when the env var is already set by the user (never overwritten);
// otherwise mints one from GOOGLE_APPLICATION_CREDENTIALS and exports it.
// Returns an error only when NEITHER source is available or the exchange
// fails — callers that require a token should surface it.
func EnsureAccessToken(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	current := strings.TrimSpace(os.Getenv("GCP_ACCESS_TOKEN"))
	// A token we did not mint belongs to the user — leave it alone.
	if current != "" && current != mintedToken {
		return nil
	}
	// Our own minted token is still fresh.
	if current != "" && current == mintedToken && nowFunc().Before(mintedExp.Add(-refreshMargin)) {
		return nil
	}

	keyPath := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if keyPath == "" {
		if current != "" {
			return nil // expiring minted token but no key to refresh with; keep it
		}
		return fmt.Errorf("set GCP_ACCESS_TOKEN (e.g. gcloud auth print-access-token) or GOOGLE_APPLICATION_CREDENTIALS (service-account key file)")
	}

	token, expiresIn, err := mintFromKeyFile(ctx, keyPath)
	if err != nil {
		return fmt.Errorf("gcpauth: exchange service-account key: %w", err)
	}
	mintedToken = token
	mintedExp = nowFunc().Add(time.Duration(expiresIn) * time.Second)
	return os.Setenv("GCP_ACCESS_TOKEN", token)
}

func mintFromKeyFile(ctx context.Context, path string) (string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	var key serviceAccountKey
	if err := json.Unmarshal(data, &key); err != nil {
		return "", 0, fmt.Errorf("parse key file: %w", err)
	}
	if key.ClientEmail == "" || key.PrivateKey == "" {
		return "", 0, fmt.Errorf("key file missing client_email/private_key")
	}
	tokenURI := key.TokenURI
	if tokenURI == "" {
		tokenURI = defaultTokenURI
	}

	assertion, err := signJWT(key, tokenURI)
	if err != nil {
		return "", 0, err
	}

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token endpoint %s: %s", resp.Status, string(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, err
	}
	if out.AccessToken == "" {
		return "", 0, fmt.Errorf("token endpoint returned no access_token")
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 3600
	}
	return out.AccessToken, out.ExpiresIn, nil
}

// signJWT builds the RS256 JWT-bearer assertion for the service account.
func signJWT(key serviceAccountKey, audience string) (string, error) {
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("private_key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Older keys use PKCS1.
		if rsaKey, err1 := x509.ParsePKCS1PrivateKey(block.Bytes); err1 == nil {
			parsed = rsaKey
		} else {
			return "", fmt.Errorf("parse private key: %w", err)
		}
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	now := nowFunc()
	header := b64json(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims := b64json(map[string]any{
		"iss":   key.ClientEmail,
		"scope": Scope,
		"aud":   audience,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signingInput := header + "." + claims
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func b64json(v any) string {
	data, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(data)
}
