package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/runfabric/runfabric/internal/cli/common"
)

func TestAuth_LoginWhoamiTokenAndLogoutFlow(t *testing.T) {
	t.Setenv("RUNFABRIC_AUTH_URL", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/device/code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-code",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://auth.example.com/device",
				"verification_uri_complete": "https://auth.example.com/device?user_code=ABCD-EFGH",
				"expires_in":                900,
				"interval":                  1,
			})
		case "/oauth/device/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-1",
				"refresh_token": "refresh-1",
				"token_type":    "Bearer",
				"expires_in":    900,
				"scope":         "openid profile email registry:read registry:write",
			})
		case "/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":   "user-123",
				"email": "dev@runfabric.test",
				"name":  "RunFabric Dev",
			})
		case "/oauth/revoke":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "/auth/logout":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// login
	loginOut := &bytes.Buffer{}
	loginCmd := newLoginCmd(&common.GlobalOptions{JSONOutput: true})
	loginCmd.SetOut(loginOut)
	loginCmd.SetErr(&bytes.Buffer{})
	loginCmd.SetArgs([]string{"--auth-url", server.URL})
	if err := loginCmd.Execute(); err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// whoami
	whoOut := &bytes.Buffer{}
	whoCmd := newWhoAmICmd(&common.GlobalOptions{JSONOutput: true})
	whoCmd.SetOut(whoOut)
	whoCmd.SetErr(&bytes.Buffer{})
	whoCmd.SetArgs([]string{"--auth-url", server.URL})
	if err := whoCmd.Execute(); err != nil {
		t.Fatalf("whoami failed: %v", err)
	}

	// token list
	listOut := &bytes.Buffer{}
	listCmd := newTokenListCmd(&common.GlobalOptions{JSONOutput: true})
	listCmd.SetOut(listOut)
	listCmd.SetErr(&bytes.Buffer{})
	listCmd.SetArgs([]string{})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("token list failed: %v", err)
	}
	if !bytes.Contains(listOut.Bytes(), []byte("tokens")) {
		t.Fatalf("expected token list JSON output, got: %s", listOut.String())
	}

	// token revoke
	revokeOut := &bytes.Buffer{}
	revokeCmd := newTokenRevokeCmd(&common.GlobalOptions{JSONOutput: true})
	revokeCmd.SetOut(revokeOut)
	revokeCmd.SetErr(&bytes.Buffer{})
	revokeCmd.SetArgs([]string{"--all", "--auth-url", server.URL})
	if err := revokeCmd.Execute(); err != nil {
		t.Fatalf("token revoke failed: %v", err)
	}

	// logout should still succeed when no tokens remain
	logoutOut := &bytes.Buffer{}
	logoutCmd := newLogoutCmd(&common.GlobalOptions{JSONOutput: true})
	logoutCmd.SetOut(logoutOut)
	logoutCmd.SetErr(&bytes.Buffer{})
	logoutCmd.SetArgs([]string{"--auth-url", server.URL})
	if err := logoutCmd.Execute(); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
}

func TestAuth_WhoAmIRefreshesExpiredToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RUNFABRIC_AUTH_URL", "")

	refreshCalled := 0
	meCalled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			refreshCalled++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-2",
				"refresh_token": "refresh-2",
				"token_type":    "Bearer",
				"expires_in":    900,
				"scope":         "openid profile email registry:read registry:write",
			})
		case "/me":
			meCalled++
			if got := r.Header.Get("Authorization"); got != "Bearer access-2" {
				t.Fatalf("expected refreshed access token, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sub": "user-123", "email": "dev@runfabric.test"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := &authTokenStore{
		ActiveID: "tok-expired",
		Tokens: []authTokenRecord{
			{
				ID:           "tok-expired",
				AuthURL:      server.URL,
				ClientID:     defaultClientID,
				AccessToken:  "expired-access",
				RefreshToken: "refresh-1",
				TokenType:    "Bearer",
				Scope:        "openid profile email",
				ExpiresAt:    time.Now().UTC().Add(-2 * time.Minute),
				CreatedAt:    time.Now().UTC().Add(-10 * time.Minute),
			},
		},
	}
	if err := saveAuthTokenStore(store); err != nil {
		t.Fatalf("seed token store: %v", err)
	}

	out := &bytes.Buffer{}
	whoCmd := newWhoAmICmd(&common.GlobalOptions{JSONOutput: true})
	whoCmd.SetOut(out)
	whoCmd.SetErr(&bytes.Buffer{})
	whoCmd.SetArgs([]string{"--auth-url", server.URL})
	if err := whoCmd.Execute(); err != nil {
		t.Fatalf("whoami failed: %v", err)
	}

	if refreshCalled == 0 {
		t.Fatal("expected refresh token endpoint to be called")
	}
	if meCalled == 0 {
		t.Fatal("expected /me endpoint to be called")
	}

	updated, err := loadAuthTokenStore()
	if err != nil {
		t.Fatalf("load updated store: %v", err)
	}
	active := updated.active()
	if active == nil {
		t.Fatal("expected active token after whoami")
	}
	if active.AccessToken != "access-2" {
		t.Fatalf("expected refreshed access token, got %q", active.AccessToken)
	}
}

func TestAuth_TokenStoreFilePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	store := &authTokenStore{
		ActiveID: "tok-1",
		Tokens: []authTokenRecord{
			{
				ID:          "tok-1",
				AuthURL:     "https://auth.example.com",
				AccessToken: "acc",
				CreatedAt:   time.Now().UTC(),
			},
		},
	}
	if err := saveAuthTokenStore(store); err != nil {
		t.Fatalf("saveAuthTokenStore: %v", err)
	}
	path, err := authTokenStorePath()
	if err != nil {
		t.Fatalf("authTokenStorePath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat auth token store: %v", err)
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Fatalf("expected token file mode 0600, got %o", mode)
	}
}
