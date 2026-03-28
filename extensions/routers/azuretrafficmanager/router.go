package azuretrafficmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

const (
	defaultAPIBaseURL = "https://management.azure.com"
	apiVersion        = "2022-04-01"
	managedPrefix     = "runfabric-"
)

// Router reconciles external endpoints in an Azure Traffic Manager profile.
type Router struct{}

func NewRouter() sdkrouter.Router {
	return Router{}
}

func RouterMeta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{
		ID:          "azure-traffic-manager",
		Name:        "Azure Traffic Manager Router",
		Description: "Azure Traffic Manager endpoint reconciler",
	}
}

func (Router) Meta() sdkrouter.PluginMeta {
	return RouterMeta()
}

func (Router) Sync(ctx context.Context, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	if req.Routing == nil {
		return nil, fmt.Errorf("routing config is nil")
	}
	profileID := resolveProfileID(req.ZoneID)
	if profileID == "" {
		return nil, fmt.Errorf("Azure Traffic Manager profile resource ID is required (pass --zone-id or set AZURE_TRAFFIC_MANAGER_PROFILE_ID)")
	}
	token := resolveToken()
	if token == "" {
		return nil, fmt.Errorf("Azure access token is required (set RUNFABRIC_ROUTER_API_TOKEN or AZURE_ACCESS_TOKEN)")
	}
	client := &httpAPIClient{
		baseURL:    resolveBaseURL(),
		token:      token,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
	return syncWithClient(ctx, client, profileID, req)
}

type apiClient interface {
	GetProfile(ctx context.Context, profileID string) (*profile, error)
	UpsertEndpoint(ctx context.Context, profileID string, in endpointResource) error
}

func syncWithClient(ctx context.Context, client apiClient, profileID string, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	desired, failover, err := desiredEndpoints(req.Routing)
	if err != nil {
		return nil, err
	}
	existingProfile, err := client.GetProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}

	existingByName := make(map[string]endpointResource, len(existingProfile.Properties.Endpoints))
	for _, ep := range existingProfile.Properties.Endpoints {
		name := normalizeEndpointName(ep.Name)
		if name == "" {
			continue
		}
		existingByName[name] = ep
	}

	result := &sdkrouter.RouterSyncResult{DryRun: req.DryRun}
	for _, want := range desired {
		name := normalizeEndpointName(want.Name)
		got, found := existingByName[name]
		action := "create"
		if found {
			if endpointEqual(got, want, failover) {
				result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
					Resource: "traffic_manager_endpoint",
					Action:   "no-op",
					Name:     name,
					Detail:   describeEndpoint(want, failover),
				})
				continue
			}
			action = "update"
		}

		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "traffic_manager_endpoint",
			Action:   action,
			Name:     name,
			Detail:   describeEndpoint(want, failover),
		})
		if req.DryRun {
			continue
		}
		if err := client.UpsertEndpoint(ctx, profileID, want); err != nil {
			return nil, err
		}
	}

	desiredNames := make(map[string]struct{}, len(desired))
	for _, ep := range desired {
		desiredNames[normalizeEndpointName(ep.Name)] = struct{}{}
	}
	for _, ep := range existingProfile.Properties.Endpoints {
		name := normalizeEndpointName(ep.Name)
		if !strings.HasPrefix(name, managedPrefix) {
			continue
		}
		if _, keep := desiredNames[name]; keep {
			continue
		}
		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "traffic_manager_endpoint",
			Action:   "delete-candidate",
			Name:     name,
			Detail:   fmt.Sprintf("azure-traffic-manager target=%s", normalizeTarget(ep.Properties.Target)),
		})
	}

	sort.Slice(result.Actions, func(i, j int) bool {
		if result.Actions[i].Action == result.Actions[j].Action {
			return result.Actions[i].Name < result.Actions[j].Name
		}
		return result.Actions[i].Action < result.Actions[j].Action
	})
	return result, nil
}

type httpAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func (c *httpAPIClient) GetProfile(ctx context.Context, profileID string) (*profile, error) {
	u := strings.TrimRight(c.baseURL, "/") + normalizeProfileID(profileID) + "?api-version=" + apiVersion
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("azure traffic manager get profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure traffic manager get profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("azure traffic manager get profile failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out profile
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("azure traffic manager decode profile: %w", err)
	}
	return &out, nil
}

func (c *httpAPIClient) UpsertEndpoint(ctx context.Context, profileID string, in endpointResource) error {
	u := strings.TrimRight(c.baseURL, "/") + normalizeProfileID(profileID) + "/externalEndpoints/" + url.PathEscape(in.Name) + "?api-version=" + apiVersion
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("azure traffic manager marshal endpoint payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("azure traffic manager upsert endpoint request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("azure traffic manager upsert endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("azure traffic manager upsert endpoint failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return nil
}

type profile struct {
	Properties profileProperties `json:"properties"`
}

type profileProperties struct {
	Endpoints []endpointResource `json:"endpoints"`
}

type endpointResource struct {
	Name       string             `json:"name"`
	Type       string             `json:"type,omitempty"`
	Properties endpointProperties `json:"properties"`
}

type endpointProperties struct {
	Target         string `json:"target"`
	EndpointStatus string `json:"endpointStatus,omitempty"`
	Weight         int    `json:"weight,omitempty"`
	Priority       int    `json:"priority,omitempty"`
}

func desiredEndpoints(routing *sdkrouter.RoutingConfig) ([]endpointResource, bool, error) {
	if strings.TrimSpace(routing.Hostname) == "" {
		return nil, false, fmt.Errorf("routing hostname is required")
	}
	if len(routing.Endpoints) == 0 {
		return nil, false, fmt.Errorf("routing endpoints are required")
	}
	failover := strings.EqualFold(strings.TrimSpace(routing.Strategy), "failover") && len(routing.Endpoints) == 2
	out := make([]endpointResource, 0, len(routing.Endpoints))
	for idx, ep := range routing.Endpoints {
		target, err := cnameTarget(ep.URL)
		if err != nil {
			return nil, false, fmt.Errorf("endpoint %q: %w", ep.Name, err)
		}
		item := endpointResource{
			Name: endpointName(ep.Name, idx),
			Type: "Microsoft.Network/trafficManagerProfiles/externalEndpoints",
			Properties: endpointProperties{
				Target:         target,
				EndpointStatus: "Enabled",
			},
		}
		if failover {
			item.Properties.Priority = idx + 1
		} else {
			weight := ep.Weight
			if weight <= 0 {
				weight = 1
			}
			item.Properties.Weight = weight
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, failover, nil
}

func endpointEqual(got, want endpointResource, failover bool) bool {
	if normalizeTarget(got.Properties.Target) != normalizeTarget(want.Properties.Target) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(got.Properties.EndpointStatus), strings.TrimSpace(want.Properties.EndpointStatus)) {
		return false
	}
	if failover {
		return got.Properties.Priority == want.Properties.Priority
	}
	return got.Properties.Weight == want.Properties.Weight
}

func describeEndpoint(in endpointResource, failover bool) string {
	if failover {
		return fmt.Sprintf("azure-traffic-manager target=%s priority=%d", normalizeTarget(in.Properties.Target), in.Properties.Priority)
	}
	return fmt.Sprintf("azure-traffic-manager target=%s weight=%d", normalizeTarget(in.Properties.Target), in.Properties.Weight)
}

func endpointName(name string, idx int) string {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		base = "endpoint-" + strconv.Itoa(idx+1)
	}
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, base)
	base = strings.Trim(base, "-")
	if base == "" {
		base = "endpoint-" + strconv.Itoa(idx+1)
	}
	return managedPrefix + base
}

func normalizeEndpointName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func resolveProfileID(zoneID string) string {
	if z := strings.TrimSpace(zoneID); z != "" {
		return normalizeProfileID(z)
	}
	if p := strings.TrimSpace(os.Getenv("AZURE_TRAFFIC_MANAGER_PROFILE_ID")); p != "" {
		return normalizeProfileID(p)
	}
	return ""
}

func normalizeProfileID(id string) string {
	out := strings.TrimSpace(id)
	if out == "" {
		return ""
	}
	if !strings.HasPrefix(out, "/") {
		out = "/" + out
	}
	return strings.TrimSuffix(out, "/")
}

func resolveToken() string {
	for _, key := range []string{"RUNFABRIC_ROUTER_API_TOKEN", "AZURE_ACCESS_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func resolveBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("AZURE_API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultAPIBaseURL
}

func normalizeTarget(target string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(target)), ".")
}

func cnameTarget(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", fmt.Errorf("endpoint URL is empty")
	}
	parsed, err := url.Parse(target)
	if err == nil && parsed.Host != "" {
		target = parsed.Host
	}
	target = normalizeTarget(target)
	if target == "" {
		return "", fmt.Errorf("endpoint URL %q has no host", raw)
	}
	if strings.Contains(target, "/") {
		return "", fmt.Errorf("endpoint URL %q resolves to invalid hostname %q", raw, target)
	}
	return target, nil
}
