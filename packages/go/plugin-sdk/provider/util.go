package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultClient is used by provider implementations for simple HTTP API operations.
var DefaultClient = &http.Client{Timeout: 30 * time.Second}

// Env reads an environment variable by key.
func Env(key string) string {
	return strings.TrimSpace(os.Getenv(strings.TrimSpace(key)))
}

// ReceiptView is a minimal receipt shape used by providers.
type ReceiptView struct {
	Outputs  map[string]string
	Metadata map[string]string
}

// DecodeReceipt extracts Outputs/Metadata from loosely typed receipt values.
func DecodeReceipt(receipt any) ReceiptView {
	out := ReceiptView{Outputs: map[string]string{}, Metadata: map[string]string{}}
	if receipt == nil {
		return out
	}
	if rv, ok := receipt.(ReceiptView); ok {
		if rv.Outputs != nil {
			out.Outputs = rv.Outputs
		}
		if rv.Metadata != nil {
			out.Metadata = rv.Metadata
		}
		return out
	}
	if m, ok := receipt.(map[string]any); ok {
		if om, ok := m["outputs"].(map[string]any); ok {
			for k, v := range om {
				out.Outputs[k] = fmt.Sprint(v)
			}
		}
		if mm, ok := m["metadata"].(map[string]any); ok {
			for k, v := range mm {
				out.Metadata[k] = fmt.Sprint(v)
			}
		}
	}
	return out
}

// APIPost issues a JSON POST with bearer auth token loaded from tokenEnv.
func APIPost(ctx context.Context, url, tokenEnv string, payload any, out any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := Env(tokenEnv); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", resp.Status, string(body))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return err
		}
	}
	return nil
}

// APIGet issues an authenticated GET and decodes JSON into out.
func APIGet(ctx context.Context, url, tokenEnv string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token := Env(tokenEnv); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", resp.Status, string(body))
	}
	if out != nil && len(body) > 0 {
		return json.Unmarshal(body, out)
	}
	return nil
}

// APIPut issues an authenticated PUT request and returns the response body.
func APIPut(ctx context.Context, url, tokenEnv string, body []byte, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token := Env(tokenEnv); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(out))
	}
	return out, nil
}

// DoDelete issues an authenticated DELETE with bearer token from tokenEnv.
func DoDelete(ctx context.Context, url, tokenEnv string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if token := Env(tokenEnv); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", resp.Status, string(body))
	}
	return nil
}

// BuildDeployResult creates a default deploy result using the SDK config map.
func BuildDeployResult(providerID string, cfg Config, stage string) *DeployResult {
	service := Service(cfg)
	if service == "" {
		service = "service"
	}
	res := &DeployResult{
		Provider:     providerID,
		DeploymentID: fmt.Sprintf("%s-%s-%d", providerID, stage, time.Now().Unix()),
		Outputs:      map[string]string{},
		Metadata:     map[string]string{},
		Functions:    map[string]DeployedFunction{},
	}
	defaultRuntime := ProviderRuntime(cfg)
	for name, fn := range Functions(cfg) {
		runtime := fn.Runtime
		if runtime == "" {
			runtime = defaultRuntime
		}
		res.Artifacts = append(res.Artifacts, Artifact{Function: name, Runtime: runtime})
		res.Functions[name] = DeployedFunction{ResourceName: name}
	}
	return res
}

// PrepareLifecycleDevStream prepares a generic lifecycle-only or route-rewrite dev-stream session.
// It uses provider-specific hook endpoints from env vars:
// RUNFABRIC_DEV_STREAM_<PROVIDER>_SET_URL, _RESTORE_URL, _TOKEN.
func PrepareLifecycleDevStream(providerName string, cfg Config, stage, tunnelURL string) (*DevStreamSession, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	service := Service(cfg)
	if strings.TrimSpace(service) == "" {
		return nil, fmt.Errorf("service required")
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
	prefix := devStreamEnvPrefix(providerName)
	setURLKey := "RUNFABRIC_DEV_STREAM_" + prefix + "_SET_URL"
	restoreURLKey := "RUNFABRIC_DEV_STREAM_" + prefix + "_RESTORE_URL"
	tokenKey := "RUNFABRIC_DEV_STREAM_" + prefix + "_TOKEN"
	setURL := Env(setURLKey)
	restoreURL := Env(restoreURLKey)
	if setURL == "" || restoreURL == "" {
		missing := []string{}
		if setURL == "" {
			missing = append(missing, setURLKey)
		}
		if restoreURL == "" {
			missing = append(missing, restoreURLKey)
		}
		return NewDevStreamSession("lifecycle-only", missing, fmt.Sprintf("falling back to lifecycle-only: gateway rewrite hooks are not fully configured (%s)", strings.Join(missing, ", ")), nil), nil
	}
	payload := map[string]string{
		"provider":  providerName,
		"service":   service,
		"stage":     stage,
		"tunnelUrl": tunnelURL,
	}
	if err := APIPost(context.Background(), setURL, tokenKey, payload, nil); err != nil {
		return NewDevStreamSession("lifecycle-only", nil, fmt.Sprintf("falling back to lifecycle-only: gateway rewrite set hook failed: %v", err), nil), nil
	}
	restore := func(ctx context.Context) error {
		return APIPost(ctx, restoreURL, tokenKey, payload, nil)
	}
	return NewDevStreamSession("route-rewrite", nil, "gateway-owned route rewrite applied via provider dev-stream hooks; routing will be restored on exit", restore), nil
}

// Service returns cfg.service when available.
func Service(cfg Config) string {
	return asString(first(cfg, "service", "Service"))
}

// ProviderRuntime returns cfg.provider.runtime when available.
func ProviderRuntime(cfg Config) string {
	m := asMap(first(cfg, "provider", "Provider"))
	return asString(first(m, "runtime", "Runtime"))
}

// ProviderRegion returns cfg.provider.region when available.
func ProviderRegion(cfg Config) string {
	m := asMap(first(cfg, "provider", "Provider"))
	return asString(first(m, "region", "Region"))
}

// FunctionView exposes commonly used function fields from the schema-free config.
type FunctionView struct {
	Handler     string
	Runtime     string
	Memory      int
	Timeout     int
	Environment map[string]string
	HasHTTP     bool
}

// Functions returns cfg.functions parsed as a typed map for convenience.
func Functions(cfg Config) map[string]FunctionView {
	out := map[string]FunctionView{}
	for name, raw := range asMap(first(cfg, "functions", "Functions")) {
		m := asMap(raw)
		out[name] = FunctionView{
			Handler:     asString(first(m, "handler", "Handler")),
			Runtime:     asString(first(m, "runtime", "Runtime")),
			Memory:      asInt(first(m, "memory", "Memory")),
			Timeout:     asInt(first(m, "timeout", "Timeout")),
			Environment: asStringStringMap(first(m, "environment", "Environment")),
			HasHTTP:     hasHTTPExposure(m),
		}
	}
	return out
}

func hasHTTPExposure(fn map[string]any) bool {
	for _, raw := range asSlice(first(fn, "events", "Events")) {
		em := asMap(raw)
		if len(em) == 0 {
			continue
		}
		if v, ok := em["http"]; ok && hasValue(v) {
			return true
		}
		if v, ok := em["HTTP"]; ok && hasValue(v) {
			return true
		}
	}
	for _, raw := range asSlice(first(fn, "triggers", "Triggers")) {
		tm := asMap(raw)
		if len(tm) == 0 {
			continue
		}
		if strings.EqualFold(asString(first(tm, "type", "Type")), "http") {
			return true
		}
	}
	return false
}

func hasValue(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) != ""
	case map[string]any:
		return len(t) > 0
	case map[string]string:
		return len(t) > 0
	default:
		return true
	}
}

func first(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func asMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(map[string]string); ok {
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = val
		}
		return out
	}
	return map[string]any{}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func asInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func asStringStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]string); ok {
		if len(m) == 0 {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, val := range m {
			out[strings.TrimSpace(k)] = strings.TrimSpace(val)
		}
		return out
	}
	raw := asMap(v)
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = asString(val)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	if s, ok := v.([]map[string]any); ok {
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	}
	if s, ok := v.([]map[string]string); ok {
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	}
	return nil
}

func devStreamEnvPrefix(providerName string) string {
	upper := strings.ToUpper(strings.TrimSpace(providerName))
	if upper == "" {
		return "PROVIDER"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range upper {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "PROVIDER"
	}
	return out
}
