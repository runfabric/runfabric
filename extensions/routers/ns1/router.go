package ns1

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
	defaultBaseURL = "https://api.nsone.net/v1"
)

// Router reconciles routing records through the NS1 API.
type Router struct{}

func NewRouter() sdkrouter.Router {
	return Router{}
}

func RouterMeta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{
		ID:          "ns1",
		Name:        "NS1 Router",
		Description: "NS1 DNS reconciler",
	}
}

func (Router) Meta() sdkrouter.PluginMeta {
	return RouterMeta()
}

func (Router) Sync(ctx context.Context, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	if req.Routing == nil {
		return nil, fmt.Errorf("routing config is nil")
	}
	zone := normalizeZone(req.ZoneID)
	if zone == "" {
		return nil, fmt.Errorf("NS1 zone is required (pass --zone-id)")
	}
	token := resolveToken()
	if token == "" {
		return nil, fmt.Errorf("NS1 API token is required (set RUNFABRIC_ROUTER_API_TOKEN or NS1_API_KEY)")
	}
	client := &httpAPIClient{
		baseURL:    resolveBaseURL(),
		token:      token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	return syncWithClient(ctx, client, zone, req)
}

type apiClient interface {
	GetRecord(ctx context.Context, zone, domain, rrType string) (*record, error)
	UpsertRecord(ctx context.Context, zone string, in record) error
}

func syncWithClient(ctx context.Context, client apiClient, zone string, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	desired, err := desiredRecord(zone, req.Routing)
	if err != nil {
		return nil, err
	}
	existing, err := client.GetRecord(ctx, zone, desired.Domain, desired.Type)
	if err != nil {
		return nil, err
	}
	result := &sdkrouter.RouterSyncResult{DryRun: req.DryRun}
	if existing == nil {
		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "dns_record_set",
			Action:   "create",
			Name:     desired.Domain,
			Detail:   describeRecord(desired),
		})
		if req.DryRun {
			return result, nil
		}
		if err := client.UpsertRecord(ctx, zone, desired); err != nil {
			return nil, err
		}
		return result, nil
	}

	for _, candidate := range deleteCandidates(*existing, desired) {
		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "dns_record_set",
			Action:   "delete-candidate",
			Name:     desired.Domain,
			Detail:   fmt.Sprintf("ns1 cname=%s", candidate),
		})
	}

	if recordsEqual(*existing, desired) {
		result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
			Resource: "dns_record_set",
			Action:   "no-op",
			Name:     desired.Domain,
			Detail:   describeRecord(desired),
		})
		return result, nil
	}

	result.Actions = append(result.Actions, sdkrouter.RouterSyncAction{
		Resource: "dns_record_set",
		Action:   "update",
		Name:     desired.Domain,
		Detail:   describeRecord(desired),
	})
	if req.DryRun {
		return result, nil
	}
	if err := client.UpsertRecord(ctx, zone, desired); err != nil {
		return nil, err
	}
	return result, nil
}

type httpAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func (c *httpAPIClient) GetRecord(ctx context.Context, zone, domain, rrType string) (*record, error) {
	path := fmt.Sprintf("%s/zones/%s/%s/%s", strings.TrimRight(c.baseURL, "/"), url.PathEscape(zone), url.PathEscape(domain), strings.ToUpper(strings.TrimSpace(rrType)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("ns1 get record request: %w", err)
	}
	req.Header.Set("X-NSONE-Key", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ns1 get record: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ns1 get record failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out record
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ns1 decode get record: %w", err)
	}
	return &out, nil
}

func (c *httpAPIClient) UpsertRecord(ctx context.Context, zone string, in record) error {
	path := fmt.Sprintf("%s/zones/%s/%s/%s", strings.TrimRight(c.baseURL, "/"), url.PathEscape(zone), url.PathEscape(in.Domain), strings.ToUpper(strings.TrimSpace(in.Type)))
	payload, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("ns1 marshal upsert payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ns1 upsert record request: %w", err)
	}
	req.Header.Set("X-NSONE-Key", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ns1 upsert record: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ns1 upsert record failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type record struct {
	Zone    string   `json:"zone,omitempty"`
	Domain  string   `json:"domain"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Answers []answer `json:"answers"`
}

type answer struct {
	Answer []string       `json:"answer"`
	Meta   map[string]any `json:"meta,omitempty"`
}

func desiredRecord(zone string, routing *sdkrouter.RoutingConfig) (record, error) {
	if strings.TrimSpace(routing.Hostname) == "" {
		return record{}, fmt.Errorf("routing hostname is required")
	}
	if len(routing.Endpoints) == 0 {
		return record{}, fmt.Errorf("routing endpoints are required")
	}
	ttl := routing.TTL
	if ttl <= 0 {
		ttl = 60
	}
	out := record{
		Zone:    zone,
		Domain:  normalizeDomain(routing.Hostname),
		Type:    "CNAME",
		TTL:     ttl,
		Answers: make([]answer, 0, len(routing.Endpoints)),
	}
	for idx, ep := range routing.Endpoints {
		target, err := cnameTarget(ep.URL)
		if err != nil {
			return record{}, fmt.Errorf("endpoint %q: %w", ep.Name, err)
		}
		meta := map[string]any{"up": true}
		if strings.EqualFold(strings.TrimSpace(routing.Strategy), "failover") && len(routing.Endpoints) == 2 {
			meta["priority"] = idx + 1
		} else {
			weight := ep.Weight
			if weight <= 0 {
				weight = 1
			}
			meta["weight"] = weight
		}
		out.Answers = append(out.Answers, answer{
			Answer: []string{target},
			Meta:   meta,
		})
	}
	sort.Slice(out.Answers, func(i, j int) bool {
		return answerKey(out.Answers[i]) < answerKey(out.Answers[j])
	})
	return out, nil
}

func recordsEqual(a, b record) bool {
	if normalizeDomain(a.Domain) != normalizeDomain(b.Domain) {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(a.Type)) != strings.ToUpper(strings.TrimSpace(b.Type)) {
		return false
	}
	if a.TTL != b.TTL {
		return false
	}
	if len(a.Answers) != len(b.Answers) {
		return false
	}
	left := append([]answer(nil), a.Answers...)
	right := append([]answer(nil), b.Answers...)
	sort.Slice(left, func(i, j int) bool { return answerKey(left[i]) < answerKey(left[j]) })
	sort.Slice(right, func(i, j int) bool { return answerKey(right[i]) < answerKey(right[j]) })
	for i := range left {
		if answerKey(left[i]) != answerKey(right[i]) {
			return false
		}
	}
	return true
}

func deleteCandidates(existing, desired record) []string {
	desiredTargets := make(map[string]struct{}, len(desired.Answers))
	for _, a := range desired.Answers {
		if len(a.Answer) == 0 {
			continue
		}
		desiredTargets[normalizeTarget(a.Answer[0])] = struct{}{}
	}
	candidates := make([]string, 0)
	for _, a := range existing.Answers {
		if len(a.Answer) == 0 {
			continue
		}
		target := normalizeTarget(a.Answer[0])
		if _, ok := desiredTargets[target]; ok {
			continue
		}
		candidates = append(candidates, target)
	}
	sort.Strings(candidates)
	return candidates
}

func describeRecord(r record) string {
	return fmt.Sprintf("ns1 cname targets=%d ttl=%d", len(r.Answers), r.TTL)
}

func answerKey(a answer) string {
	target := ""
	if len(a.Answer) > 0 {
		target = normalizeTarget(a.Answer[0])
	}
	weight := toInt(a.Meta["weight"])
	priority := toInt(a.Meta["priority"])
	return target + "|w=" + strconv.Itoa(weight) + "|p=" + strconv.Itoa(priority)
}

func toInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float32:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

func resolveToken() string {
	for _, k := range []string{"RUNFABRIC_ROUTER_API_TOKEN", "NS1_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func resolveBaseURL() string {
	base := strings.TrimSpace(os.Getenv("NS1_API_BASE_URL"))
	if base == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(base, "/")
}

func normalizeZone(zone string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zone)), ".")
}

func normalizeDomain(domain string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
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
