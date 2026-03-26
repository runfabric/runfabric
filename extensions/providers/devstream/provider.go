package devstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

var defaultHTTPClient = &http.Client{}

// LifecycleState captures provider-neutral dev-stream lifecycle hook state.
// Providers without a safe reversible route rewrite use this as an explicit
// lifecycle-only contract so the CLI can still offer consistent prepare/restore flow.
type LifecycleState struct {
	ProviderName string
	Service      string
	Stage        string
	TunnelURL    string
	Mode         string

	GatewaySetURL     string
	GatewayRestoreURL string
	GatewayTokenEnv   string
	GatewayApplied    bool

	MissingPrereqs []string
	StatusMessage  string
}

// RedirectToTunnel validates the lifecycle-hook request and returns a stable restore handle.
func RedirectToTunnel(providerName string, cfg sdkprovider.Config, stage, tunnelURL string) (*LifecycleState, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	service, err := serviceFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(providerName) == "" {
		return nil, fmt.Errorf("provider name required")
	}
	if strings.TrimSpace(stage) == "" {
		return nil, fmt.Errorf("stage required")
	}
	if strings.TrimSpace(tunnelURL) == "" {
		return nil, fmt.Errorf("tunnel URL required")
	}
	state := &LifecycleState{
		ProviderName:    providerName,
		Service:         service,
		Stage:           stage,
		TunnelURL:       tunnelURL,
		Mode:            "lifecycle-only",
		GatewayTokenEnv: gatewayHookTokenEnv(providerName),
	}

	setURL, restoreURL := gatewayHookURLs(providerName)
	state.GatewaySetURL = strings.TrimSpace(os.Getenv(setURL))
	state.GatewayRestoreURL = strings.TrimSpace(os.Getenv(restoreURL))
	if state.GatewaySetURL == "" || state.GatewayRestoreURL == "" {
		if state.GatewaySetURL == "" {
			state.MissingPrereqs = append(state.MissingPrereqs, setURL)
		}
		if state.GatewayRestoreURL == "" {
			state.MissingPrereqs = append(state.MissingPrereqs, restoreURL)
		}
		state.StatusMessage = fmt.Sprintf("lifecycle-only fallback: gateway rewrite hooks are not fully configured (%s)", strings.Join(state.MissingPrereqs, ", "))
		return state, nil
	}

	payload := map[string]string{
		"provider":  providerName,
		"service":   service,
		"stage":     stage,
		"tunnelUrl": tunnelURL,
	}
	if err := postJSON(context.Background(), state.GatewaySetURL, state.GatewayTokenEnv, payload); err != nil {
		state.StatusMessage = fmt.Sprintf("lifecycle-only fallback: gateway rewrite set hook failed: %v", err)
		return state, nil
	}

	state.GatewayApplied = true
	state.Mode = "route-rewrite"
	state.StatusMessage = "gateway-owned route rewrite applied via provider dev-stream hooks; routing will be restored on exit"
	return state, nil
}

// Restore calls the gateway restore hook when a route rewrite was applied.
func (s *LifecycleState) Restore(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if !s.GatewayApplied || strings.TrimSpace(s.GatewayRestoreURL) == "" {
		return nil
	}
	payload := map[string]string{
		"provider":  s.ProviderName,
		"service":   s.Service,
		"stage":     s.Stage,
		"tunnelUrl": s.TunnelURL,
	}
	if err := postJSON(ctx, s.GatewayRestoreURL, s.GatewayTokenEnv, payload); err != nil {
		return fmt.Errorf("%s dev-stream restore failed: %w", s.ProviderName, err)
	}
	s.GatewayApplied = false
	return nil
}

func postJSON(ctx context.Context, url, tokenEnv string, payload map[string]string) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(url), strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv(strings.TrimSpace(tokenEnv))); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func serviceFromConfig(cfg sdkprovider.Config) (string, error) {
	v, ok := cfg["service"]
	if !ok {
		return "", fmt.Errorf("service missing from config")
	}
	service := strings.TrimSpace(fmt.Sprint(v))
	if service == "" {
		return "", fmt.Errorf("service missing from config")
	}
	return service, nil
}

func gatewayHookURLs(providerName string) (setURL string, restoreURL string) {
	prefix := gatewayHookEnvPrefix(providerName)
	return "RUNFABRIC_DEV_STREAM_" + prefix + "_SET_URL", "RUNFABRIC_DEV_STREAM_" + prefix + "_RESTORE_URL"
}

func gatewayHookTokenEnv(providerName string) string {
	return "RUNFABRIC_DEV_STREAM_" + gatewayHookEnvPrefix(providerName) + "_TOKEN"
}

func gatewayHookEnvPrefix(providerName string) string {
	upper := strings.ToUpper(strings.TrimSpace(providerName))
	if upper == "" {
		return "PROVIDER"
	}
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	prefix := re.ReplaceAllString(upper, "_")
	prefix = strings.Trim(prefix, "_")
	if prefix == "" {
		return "PROVIDER"
	}
	return prefix
}
