package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	envAuthURL        = "RUNFABRIC_AUTH_URL"
	defaultAuthURL    = "https://auth.runfabric.cloud"
	defaultClientID   = "runfabric-cli"
	defaultAuthScopes = "openid profile email offline_access registry:read registry:write"
)

type authTokenRecord struct {
	ID           string    `json:"id"`
	AuthURL      string    `json:"authUrl"`
	ClientID     string    `json:"clientId"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	TokenType    string    `json:"tokenType,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	LastUsedAt   time.Time `json:"lastUsedAt,omitempty"`
	Subject      string    `json:"subject,omitempty"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
}

type authTokenStore struct {
	ActiveID string            `json:"activeId,omitempty"`
	Tokens   []authTokenRecord `json:"tokens,omitempty"`
}

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func newAuthCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Token operations (list, revoke)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTokenListCmd(opts), newTokenRevokeCmd(opts))
	return cmd
}

func newLoginCmd(opts *GlobalOptions) *cobra.Command {
	var authURL string
	var clientID string
	var scope string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate the CLI using OAuth device code flow",
		RunE: func(c *cobra.Command, args []string) error {
			resolvedAuthURL := resolveAuthURL(authURL)
			client := &http.Client{Timeout: 30 * time.Second}

			statusRunning(opts.JSONOutput, "Requesting device code...")
			dc, err := requestDeviceCode(client, resolvedAuthURL, clientID, scope)
			if err != nil {
				statusFail(opts.JSONOutput, "Login failed.")
				return err
			}

			if !opts.JSONOutput {
				fmt.Fprintln(c.OutOrStdout(), "Open the verification URL and complete sign-in:")
				if strings.TrimSpace(dc.VerificationURIComplete) != "" {
					fmt.Fprintf(c.OutOrStdout(), "  %s\n", dc.VerificationURIComplete)
				} else {
					fmt.Fprintf(c.OutOrStdout(), "  %s\n", dc.VerificationURI)
				}
				fmt.Fprintf(c.OutOrStdout(), "User code: %s\n", dc.UserCode)
			}

			statusRunning(opts.JSONOutput, "Waiting for authorization...")
			tok, err := pollDeviceToken(client, resolvedAuthURL, clientID, dc)
			if err != nil {
				statusFail(opts.JSONOutput, "Login failed.")
				return err
			}

			record := authTokenRecord{
				ID:           fmt.Sprintf("tok_%d", time.Now().UnixNano()),
				AuthURL:      resolvedAuthURL,
				ClientID:     clientID,
				AccessToken:  tok.AccessToken,
				RefreshToken: tok.RefreshToken,
				TokenType:    tokenTypeOrBearer(tok.TokenType),
				Scope:        tok.Scope,
				CreatedAt:    time.Now().UTC(),
				LastUsedAt:   time.Now().UTC(),
			}
			if tok.ExpiresIn > 0 {
				record.ExpiresAt = time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second)
			}

			me, err := fetchWhoAmI(client, resolvedAuthURL, record.AccessToken)
			if err == nil {
				record.Subject = firstString(me, "sub", "id", "user_id")
				record.Email = firstString(me, "email")
				record.Name = firstString(me, "name", "username")
			}
			if err := saveActiveAuthToken(record); err != nil {
				statusFail(opts.JSONOutput, "Login failed.")
				return err
			}

			statusDone(opts.JSONOutput, "Login complete.")
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "login", map[string]any{
					"authUrl": resolvedAuthURL,
					"tokenId": record.ID,
					"user":    me,
				}, nil)
			}

			identity := record.Email
			if identity == "" {
				identity = record.Subject
			}
			if identity == "" {
				identity = "unknown"
			}
			fmt.Fprintf(c.OutOrStdout(), "Logged in as %s\n", identity)
			return nil
		},
	}
	cmd.Flags().StringVar(&authURL, "auth-url", "", "Auth server base URL (default from RUNFABRIC_AUTH_URL/.runfabricrc/auth.url)")
	cmd.Flags().StringVar(&clientID, "client-id", defaultClientID, "OAuth client_id for device flow")
	cmd.Flags().StringVar(&scope, "scope", defaultAuthScopes, "OAuth scope string")
	return cmd
}

func newWhoAmICmd(opts *GlobalOptions) *cobra.Command {
	var authURL string
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show current authenticated identity",
		RunE: func(c *cobra.Command, args []string) error {
			store, err := loadAuthTokenStore()
			if err != nil {
				return err
			}
			active := store.active()
			if active == nil {
				return fmt.Errorf("not logged in (run `runfabric login`)")
			}
			resolvedAuthURL := active.AuthURL
			if strings.TrimSpace(authURL) != "" {
				resolvedAuthURL = normalizeBaseURL(authURL)
			}
			client := &http.Client{Timeout: 30 * time.Second}
			accessToken, changed, err := ensureActiveAccessToken(client, resolvedAuthURL, active)
			if err != nil {
				return err
			}
			if changed {
				if err := saveAuthTokenStore(store); err != nil {
					return err
				}
			}
			me, err := fetchWhoAmI(client, resolvedAuthURL, accessToken)
			if err != nil {
				return err
			}
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "whoami", map[string]any{"user": me}, nil)
			}
			fmt.Fprintf(c.OutOrStdout(), "subject: %s\n", firstString(me, "sub", "id", "user_id"))
			if email := firstString(me, "email"); email != "" {
				fmt.Fprintf(c.OutOrStdout(), "email:   %s\n", email)
			}
			if name := firstString(me, "name", "username"); name != "" {
				fmt.Fprintf(c.OutOrStdout(), "name:    %s\n", name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&authURL, "auth-url", "", "Optional auth server override (default active login auth URL)")
	return cmd
}

func newLogoutCmd(opts *GlobalOptions) *cobra.Command {
	var authURL string
	var remote bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear local auth tokens (and optionally call remote logout endpoint)",
		RunE: func(c *cobra.Command, args []string) error {
			store, err := loadAuthTokenStore()
			if err != nil {
				return err
			}
			active := store.active()
			if active == nil {
				if opts.JSONOutput {
					return WriteJSONEnvelope(c.OutOrStdout(), true, "logout", map[string]any{"cleared": 0}, nil)
				}
				fmt.Fprintln(c.OutOrStdout(), "Already logged out.")
				return nil
			}
			resolvedAuthURL := active.AuthURL
			if strings.TrimSpace(authURL) != "" {
				resolvedAuthURL = normalizeBaseURL(authURL)
			}
			if remote {
				client := &http.Client{Timeout: 15 * time.Second}
				_ = remoteLogout(client, resolvedAuthURL, active.AccessToken)
			}
			cleared := len(store.Tokens)
			store.Tokens = nil
			store.ActiveID = ""
			if err := saveAuthTokenStore(store); err != nil {
				return err
			}
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "logout", map[string]any{"cleared": cleared}, nil)
			}
			fmt.Fprintln(c.OutOrStdout(), "Logged out.")
			return nil
		},
	}
	cmd.Flags().StringVar(&authURL, "auth-url", "", "Optional auth server override (default active login auth URL)")
	cmd.Flags().BoolVar(&remote, "remote", true, "Call remote /auth/logout before clearing local tokens")
	return cmd
}

func newTokenListCmd(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List locally stored auth tokens",
		RunE: func(c *cobra.Command, args []string) error {
			store, err := loadAuthTokenStore()
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(store.Tokens))
			for _, t := range store.Tokens {
				out = append(out, map[string]any{
					"id":        t.ID,
					"active":    t.ID == store.ActiveID,
					"authUrl":   t.AuthURL,
					"scope":     t.Scope,
					"expiresAt": t.ExpiresAt,
					"createdAt": t.CreatedAt,
					"subject":   t.Subject,
					"email":     t.Email,
					"name":      t.Name,
				})
			}
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "token list", map[string]any{"tokens": out}, nil)
			}
			if len(out) == 0 {
				fmt.Fprintln(c.OutOrStdout(), "No local tokens.")
				return nil
			}
			fmt.Fprintln(c.OutOrStdout(), "ID\tACTIVE\tEXPIRES\tEMAIL\tSCOPE")
			for _, row := range out {
				expires := "n/a"
				if tm, ok := row["expiresAt"].(time.Time); ok && !tm.IsZero() {
					expires = tm.Format(time.RFC3339)
				}
				fmt.Fprintf(c.OutOrStdout(), "%s\t%t\t%s\t%s\t%s\n",
					row["id"], row["active"], expires, strAny(row["email"]), strAny(row["scope"]))
			}
			return nil
		},
	}
}

func newTokenRevokeCmd(opts *GlobalOptions) *cobra.Command {
	var authURL string
	var all bool
	cmd := &cobra.Command{
		Use:   "revoke [token-id]",
		Short: "Revoke token(s) remotely and remove local token records",
		RunE: func(c *cobra.Command, args []string) error {
			store, err := loadAuthTokenStore()
			if err != nil {
				return err
			}
			if len(store.Tokens) == 0 {
				return fmt.Errorf("no local tokens found")
			}
			targetIDs := map[string]bool{}
			if all {
				for _, t := range store.Tokens {
					targetIDs[t.ID] = true
				}
			} else if len(args) > 0 {
				targetIDs[args[0]] = true
			} else if store.ActiveID != "" {
				targetIDs[store.ActiveID] = true
			} else {
				return fmt.Errorf("no active token; provide token id or --all")
			}

			client := &http.Client{Timeout: 20 * time.Second}
			next := make([]authTokenRecord, 0, len(store.Tokens))
			revoked := make([]string, 0, len(targetIDs))
			for _, t := range store.Tokens {
				if !targetIDs[t.ID] {
					next = append(next, t)
					continue
				}
				base := t.AuthURL
				if strings.TrimSpace(authURL) != "" {
					base = normalizeBaseURL(authURL)
				}
				_ = revokeRemoteToken(client, base, t.RefreshToken, "refresh_token")
				_ = revokeRemoteToken(client, base, t.AccessToken, "access_token")
				revoked = append(revoked, t.ID)
			}
			store.Tokens = next
			if targetIDs[store.ActiveID] {
				store.ActiveID = ""
			}
			if err := saveAuthTokenStore(store); err != nil {
				return err
			}
			if opts.JSONOutput {
				return WriteJSONEnvelope(c.OutOrStdout(), true, "token revoke", map[string]any{"revoked": revoked}, nil)
			}
			fmt.Fprintf(c.OutOrStdout(), "Revoked %d token(s).\n", len(revoked))
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Revoke all local tokens")
	cmd.Flags().StringVar(&authURL, "auth-url", "", "Optional auth server override")
	return cmd
}

func requestDeviceCode(client *http.Client, authURL, clientID, scope string) (*deviceCodeResponse, error) {
	reqBody := map[string]string{"client_id": clientID, "scope": scope}
	b, _ := json.Marshal(reqBody)
	u := normalizeBaseURL(authURL) + "/oauth/device/code"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readAPIError(resp)
	}
	var out deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.DeviceCode) == "" {
		return nil, fmt.Errorf("device code response missing device_code")
	}
	return &out, nil
}

func pollDeviceToken(client *http.Client, authURL, clientID string, dc *deviceCodeResponse) (*tokenResponse, error) {
	interval := dc.Interval
	if interval <= 0 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	if dc.ExpiresIn <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("device authorization timed out")
		}
		tok, oauthErr, err := requestDeviceToken(client, authURL, clientID, dc.DeviceCode)
		if err != nil {
			return nil, err
		}
		if tok != nil {
			return tok, nil
		}
		switch oauthErr {
		case "authorization_pending":
			time.Sleep(time.Duration(interval) * time.Second)
		case "slow_down":
			interval += 5
			time.Sleep(time.Duration(interval) * time.Second)
		case "expired_token":
			return nil, fmt.Errorf("device code expired; run `runfabric login` again")
		case "access_denied":
			return nil, fmt.Errorf("login denied by user")
		default:
			return nil, fmt.Errorf("device token error: %s", oauthErr)
		}
	}
}

func requestDeviceToken(client *http.Client, authURL, clientID, deviceCode string) (*tokenResponse, string, error) {
	reqBody := map[string]string{
		"client_id":   clientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}
	b, _ := json.Marshal(reqBody)
	u := normalizeBaseURL(authURL) + "/oauth/device/token"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var tok tokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(tok.AccessToken) == "" {
			return nil, "", fmt.Errorf("token response missing access_token")
		}
		return &tok, "", nil
	}
	var oe oauthErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&oe); err == nil && strings.TrimSpace(oe.Error) != "" {
		return nil, oe.Error, nil
	}
	return nil, "", readAPIError(resp)
}

func fetchWhoAmI(client *http.Client, authURL, accessToken string) (map[string]any, error) {
	u := normalizeBaseURL(authURL) + "/me"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readAPIError(resp)
	}
	var me map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return nil, err
	}
	return me, nil
}

func remoteLogout(client *http.Client, authURL, accessToken string) error {
	u := normalizeBaseURL(authURL) + "/auth/logout"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(accessToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}
	return nil
}

func revokeRemoteToken(client *http.Client, authURL, token, hint string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	body := map[string]string{"token": token}
	if hint != "" {
		body["token_type_hint"] = hint
	}
	b, _ := json.Marshal(body)
	u := normalizeBaseURL(authURL) + "/oauth/revoke"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}
	return nil
}

func refreshAccessToken(client *http.Client, authURL, clientID, refreshToken string) (*tokenResponse, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	}
	b, _ := json.Marshal(body)
	u := normalizeBaseURL(authURL) + "/oauth/token"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readAPIError(resp)
	}
	var out tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, fmt.Errorf("refresh response missing access_token")
	}
	return &out, nil
}

func ensureActiveAccessToken(client *http.Client, authURL string, token *authTokenRecord) (string, bool, error) {
	if token == nil {
		return "", false, fmt.Errorf("no active token")
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", false, fmt.Errorf("active token missing access token")
	}
	if token.ExpiresAt.IsZero() || time.Now().UTC().Before(token.ExpiresAt.Add(-60*time.Second)) {
		token.LastUsedAt = time.Now().UTC()
		return token.AccessToken, false, nil
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return "", false, fmt.Errorf("access token expired and no refresh token is available")
	}
	tok, err := refreshAccessToken(client, authURL, token.ClientID, token.RefreshToken)
	if err != nil {
		return "", false, err
	}
	token.AccessToken = tok.AccessToken
	if strings.TrimSpace(tok.RefreshToken) != "" {
		token.RefreshToken = tok.RefreshToken
	}
	token.TokenType = tokenTypeOrBearer(tok.TokenType)
	if tok.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	if strings.TrimSpace(tok.Scope) != "" {
		token.Scope = tok.Scope
	}
	token.LastUsedAt = time.Now().UTC()
	return token.AccessToken, true, nil
}

func loadAuthTokenStore() (*authTokenStore, error) {
	path, err := authTokenStorePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &authTokenStore{}, nil
		}
		return nil, err
	}
	var s authTokenStore
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse auth token store: %w", err)
	}
	return &s, nil
}

func saveAuthTokenStore(store *authTokenStore) error {
	path, err := authTokenStorePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func saveActiveAuthToken(record authTokenRecord) error {
	store, err := loadAuthTokenStore()
	if err != nil {
		return err
	}
	replaced := false
	for i := range store.Tokens {
		if store.Tokens[i].ID == record.ID {
			store.Tokens[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		store.Tokens = append(store.Tokens, record)
	}
	store.ActiveID = record.ID
	return saveAuthTokenStore(store)
}

func (s *authTokenStore) active() *authTokenRecord {
	if s == nil || s.ActiveID == "" {
		return nil
	}
	for i := range s.Tokens {
		if s.Tokens[i].ID == s.ActiveID {
			return &s.Tokens[i]
		}
	}
	return nil
}

func authTokenStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".runfabric", "auth", "tokens.json"), nil
}

func resolveAuthURL(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return normalizeBaseURL(explicit)
	}
	if v := strings.TrimSpace(os.Getenv(envAuthURL)); v != "" {
		return normalizeBaseURL(v)
	}
	rc := loadRunfabricrc()
	if strings.TrimSpace(rc.AuthURL) != "" {
		return normalizeBaseURL(rc.AuthURL)
	}
	if strings.TrimSpace(rc.RegistryURL) != "" {
		return normalizeBaseURL(rc.RegistryURL)
	}
	return normalizeBaseURL(defaultAuthURL)
}

func normalizeBaseURL(v string) string {
	return strings.TrimRight(strings.TrimSpace(v), "/")
}

func tokenTypeOrBearer(v string) string {
	if strings.TrimSpace(v) == "" {
		return "Bearer"
	}
	return v
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func strAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func readAPIError(resp *http.Response) error {
	var oe oauthErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&oe); err == nil && strings.TrimSpace(oe.Error) != "" {
		msg := oe.Error
		if strings.TrimSpace(oe.ErrorDescription) != "" {
			msg += ": " + strings.TrimSpace(oe.ErrorDescription)
		}
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("request failed with status %s", resp.Status)
}

func registryTokenFromAuthStore(ctx context.Context, authURL string) (string, error) {
	store, err := loadAuthTokenStore()
	if err != nil {
		return "", err
	}
	active := store.active()
	if active == nil {
		return "", nil
	}
	base := active.AuthURL
	if strings.TrimSpace(authURL) != "" {
		base = normalizeBaseURL(authURL)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	token, changed, err := ensureActiveAccessToken(client, base, active)
	if err != nil {
		return "", err
	}
	if changed {
		if err := saveAuthTokenStore(store); err != nil {
			return "", err
		}
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	return token, nil
}
