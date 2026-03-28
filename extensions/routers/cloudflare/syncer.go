package cloudflare

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

const managedByTag = "managed-by:runfabric"

type cloudflareConfig struct {
	APIToken  string
	ZoneID    string
	AccountID string
}

type cloudflareSyncer struct {
	cfg    cloudflareConfig
	client *cloudflareClient
	dryRun bool
	out    io.Writer
}

func newCloudflareSyncer(cfg cloudflareConfig, dryRun bool, out io.Writer) (*cloudflareSyncer, error) {
	if err := auditCloudflareAPIToken(cfg.APIToken); err != nil {
		return nil, err
	}
	if cfg.ZoneID == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ZONE_ID is required")
	}
	if out == nil {
		out = io.Discard
	}
	return &cloudflareSyncer{
		cfg:    cfg,
		client: &cloudflareClient{token: cfg.APIToken, zoneID: cfg.ZoneID, accountID: cfg.AccountID},
		dryRun: dryRun,
		out:    out,
	}, nil
}

func (s *cloudflareSyncer) sync(ctx context.Context, routing *sdkrouter.RoutingConfig) (*sdkrouter.RouterSyncResult, error) {
	if routing == nil {
		return nil, fmt.Errorf("routing config is nil")
	}
	if err := s.preflight(routing); err != nil {
		return nil, err
	}

	result := &sdkrouter.RouterSyncResult{DryRun: s.dryRun}

	if s.cfg.AccountID != "" {
		monitorID, action, err := s.syncMonitor(ctx, routing)
		if err != nil {
			return result, fmt.Errorf("sync monitor: %w", err)
		}
		result.Actions = append(result.Actions, action)

		poolID, action, err := s.syncPool(ctx, routing, monitorID)
		if err != nil {
			return result, fmt.Errorf("sync lb pool: %w", err)
		}
		result.Actions = append(result.Actions, action)

		action, err = s.syncLoadBalancer(ctx, routing, poolID)
		if err != nil {
			return result, fmt.Errorf("sync load balancer: %w", err)
		}
		result.Actions = append(result.Actions, action)

		candidates, err := s.collectLBDeleteCandidates(ctx, routing, monitorID, poolID)
		if err != nil {
			return result, fmt.Errorf("collect delete candidates: %w", err)
		}
		result.Actions = append(result.Actions, candidates...)
	} else {
		action, err := s.syncDNSRecord(ctx, routing)
		if err != nil {
			return result, fmt.Errorf("sync dns record: %w", err)
		}
		result.Actions = append(result.Actions, action)
		candidates, err := s.collectDNSDeleteCandidates(ctx, routing)
		if err != nil {
			return result, fmt.Errorf("collect dns delete candidates: %w", err)
		}
		result.Actions = append(result.Actions, candidates...)
	}

	s.printSummary(result)
	return result, nil
}

func (s *cloudflareSyncer) preflight(routing *sdkrouter.RoutingConfig) error {
	if routing.Hostname == "" {
		return fmt.Errorf("preflight: hostname is empty")
	}
	if len(routing.Endpoints) == 0 {
		return fmt.Errorf("preflight: no endpoints configured")
	}
	seen := make(map[string]bool, len(routing.Endpoints))
	for i, ep := range routing.Endpoints {
		if ep.URL == "" {
			return fmt.Errorf("preflight: endpoint[%d] %q has empty URL", i, ep.Name)
		}
		if _, err := url.ParseRequestURI(ep.URL); err != nil {
			return fmt.Errorf("preflight: endpoint[%d] %q invalid URL %q: %w", i, ep.Name, ep.URL, err)
		}
		if seen[ep.Name] {
			return fmt.Errorf("preflight: duplicate endpoint name %q", ep.Name)
		}
		seen[ep.Name] = true
	}
	return nil
}

func (s *cloudflareSyncer) syncMonitor(ctx context.Context, routing *sdkrouter.RoutingConfig) (string, sdkrouter.RouterSyncAction, error) {
	name := monitorName(routing)
	healthPath := routing.HealthPath
	if healthPath == "" {
		healthPath = "/health"
	}
	desired := cloudflareLBMonitor{Type: "https", Path: healthPath, Description: managedByTag + " service=" + routing.Service, Interval: 60, Timeout: 5, Retries: 2}

	existing, err := s.client.listMonitors(ctx)
	if err != nil {
		return "", sdkrouter.RouterSyncAction{}, err
	}
	for _, m := range existing {
		if m.Description == desired.Description {
			if m.Path == healthPath && m.Type == "https" {
				return m.ID, sdkrouter.RouterSyncAction{Resource: "lb_monitor", Action: "no-op", Name: name}, nil
			}
			if s.dryRun {
				return m.ID, sdkrouter.RouterSyncAction{Resource: "lb_monitor", Action: "update", Name: name, Detail: fmt.Sprintf("path updated to %s", healthPath)}, nil
			}
			updated, err := s.client.updateMonitor(ctx, m.ID, desired)
			if err != nil {
				return "", sdkrouter.RouterSyncAction{}, err
			}
			return updated.ID, sdkrouter.RouterSyncAction{Resource: "lb_monitor", Action: "update", Name: name, Detail: fmt.Sprintf("path updated to %s", healthPath)}, nil
		}
	}

	if s.dryRun {
		return "dry-run-monitor-id", sdkrouter.RouterSyncAction{Resource: "lb_monitor", Action: "create", Name: name, Detail: fmt.Sprintf("type=https path=%s interval=60s", healthPath)}, nil
	}
	created, err := s.client.createMonitor(ctx, desired)
	if err != nil {
		return "", sdkrouter.RouterSyncAction{}, err
	}
	return created.ID, sdkrouter.RouterSyncAction{Resource: "lb_monitor", Action: "create", Name: name, Detail: fmt.Sprintf("type=https path=%s interval=60s", healthPath)}, nil
}

func (s *cloudflareSyncer) syncPool(ctx context.Context, routing *sdkrouter.RoutingConfig, monitorID string) (string, sdkrouter.RouterSyncAction, error) {
	name := poolName(routing)
	desiredOrigins := toOrigins(routing.Endpoints)
	desired := cloudflareLBPool{Name: name, Description: managedByTag + " service=" + routing.Service + " stage=" + routing.Stage, Origins: desiredOrigins, Monitor: monitorID, Enabled: true}

	existing, err := s.client.listPools(ctx)
	if err != nil {
		return "", sdkrouter.RouterSyncAction{}, err
	}
	for _, p := range existing {
		if p.Name == name {
			if originsEqual(p.Origins, desiredOrigins) && p.Monitor == monitorID {
				return p.ID, sdkrouter.RouterSyncAction{Resource: "lb_pool", Action: "no-op", Name: name}, nil
			}
			if s.dryRun {
				return p.ID, sdkrouter.RouterSyncAction{Resource: "lb_pool", Action: "update", Name: name, Detail: fmt.Sprintf("%d origins", len(desiredOrigins))}, nil
			}
			updated, err := s.client.updatePool(ctx, p.ID, desired)
			if err != nil {
				return "", sdkrouter.RouterSyncAction{}, err
			}
			return updated.ID, sdkrouter.RouterSyncAction{Resource: "lb_pool", Action: "update", Name: name, Detail: fmt.Sprintf("%d origins", len(desiredOrigins))}, nil
		}
	}

	if s.dryRun {
		return "dry-run-pool-id", sdkrouter.RouterSyncAction{Resource: "lb_pool", Action: "create", Name: name, Detail: fmt.Sprintf("%d origins", len(desiredOrigins))}, nil
	}
	created, err := s.client.createPool(ctx, desired)
	if err != nil {
		return "", sdkrouter.RouterSyncAction{}, err
	}
	return created.ID, sdkrouter.RouterSyncAction{Resource: "lb_pool", Action: "create", Name: name, Detail: fmt.Sprintf("%d origins", len(desiredOrigins))}, nil
}

func (s *cloudflareSyncer) syncLoadBalancer(ctx context.Context, routing *sdkrouter.RoutingConfig, poolID string) (sdkrouter.RouterSyncAction, error) {
	name := lbName(routing)
	steering := steeringPolicy(routing.Strategy)
	desired := cloudflareLoadBalancer{Name: name, Description: managedByTag + " service=" + routing.Service + " stage=" + routing.Stage, FallbackPool: poolID, DefaultPools: []string{poolID}, SteeringPolicy: steering, TTL: routing.TTL, Proxied: true, Enabled: true}

	existing, err := s.client.listLoadBalancers(ctx)
	if err != nil {
		return sdkrouter.RouterSyncAction{}, err
	}
	for _, lb := range existing {
		if lb.Name == name {
			poolUnchanged := len(lb.DefaultPools) > 0 && lb.DefaultPools[0] == poolID
			if poolUnchanged && lb.SteeringPolicy == steering && lb.TTL == routing.TTL {
				return sdkrouter.RouterSyncAction{Resource: "load_balancer", Action: "no-op", Name: name}, nil
			}
			if s.dryRun {
				return sdkrouter.RouterSyncAction{Resource: "load_balancer", Action: "update", Name: name, Detail: fmt.Sprintf("steering=%s ttl=%d", steering, routing.TTL)}, nil
			}
			if _, err := s.client.updateLoadBalancer(ctx, lb.ID, desired); err != nil {
				return sdkrouter.RouterSyncAction{}, err
			}
			return sdkrouter.RouterSyncAction{Resource: "load_balancer", Action: "update", Name: name, Detail: fmt.Sprintf("steering=%s ttl=%d", steering, routing.TTL)}, nil
		}
	}

	if s.dryRun {
		return sdkrouter.RouterSyncAction{Resource: "load_balancer", Action: "create", Name: name, Detail: fmt.Sprintf("strategy=%s steering=%s ttl=%d", routing.Strategy, steering, routing.TTL)}, nil
	}
	if _, err := s.client.createLoadBalancer(ctx, desired); err != nil {
		return sdkrouter.RouterSyncAction{}, err
	}
	return sdkrouter.RouterSyncAction{Resource: "load_balancer", Action: "create", Name: name, Detail: fmt.Sprintf("strategy=%s steering=%s ttl=%d", routing.Strategy, steering, routing.TTL)}, nil
}

func (s *cloudflareSyncer) syncDNSRecord(ctx context.Context, routing *sdkrouter.RoutingConfig) (sdkrouter.RouterSyncAction, error) {
	target := stripScheme(routing.Endpoints[0].URL)
	desired := cloudflareDNSRecord{Type: "CNAME", Name: routing.Hostname, Content: target, TTL: routing.TTL, Proxied: false, Comment: managedByTag}

	existing, err := s.client.listDNSRecords(ctx, routing.Hostname, "CNAME")
	if err != nil {
		return sdkrouter.RouterSyncAction{}, err
	}
	for _, r := range existing {
		if r.Name == routing.Hostname {
			if r.Content == target && r.TTL == routing.TTL {
				return sdkrouter.RouterSyncAction{Resource: "dns_record", Action: "no-op", Name: routing.Hostname}, nil
			}
			if s.dryRun {
				return sdkrouter.RouterSyncAction{Resource: "dns_record", Action: "update", Name: routing.Hostname, Detail: fmt.Sprintf("CNAME -> %s (TTL %d)", target, routing.TTL)}, nil
			}
			desired.Comment = r.Comment
			if _, err := s.client.updateDNSRecord(ctx, r.ID, desired); err != nil {
				return sdkrouter.RouterSyncAction{}, err
			}
			return sdkrouter.RouterSyncAction{Resource: "dns_record", Action: "update", Name: routing.Hostname, Detail: fmt.Sprintf("CNAME -> %s (TTL %d)", target, routing.TTL)}, nil
		}
	}

	if s.dryRun {
		return sdkrouter.RouterSyncAction{Resource: "dns_record", Action: "create", Name: routing.Hostname, Detail: fmt.Sprintf("CNAME -> %s (TTL %d)", target, routing.TTL)}, nil
	}
	if _, err := s.client.createDNSRecord(ctx, desired); err != nil {
		return sdkrouter.RouterSyncAction{}, err
	}
	return sdkrouter.RouterSyncAction{Resource: "dns_record", Action: "create", Name: routing.Hostname, Detail: fmt.Sprintf("CNAME -> %s (TTL %d)", target, routing.TTL)}, nil
}

func (s *cloudflareSyncer) collectDNSDeleteCandidates(ctx context.Context, routing *sdkrouter.RoutingConfig) ([]sdkrouter.RouterSyncAction, error) {
	records, err := s.client.listDNSRecords(ctx, routing.Hostname, "CNAME")
	if err != nil {
		return nil, err
	}
	target := stripScheme(routing.Endpoints[0].URL)
	managed := make([]cloudflareDNSRecord, 0, len(records))
	for _, record := range records {
		if record.Name != routing.Hostname {
			continue
		}
		if strings.TrimSpace(record.Comment) != managedByTag {
			continue
		}
		managed = append(managed, record)
	}
	if len(managed) <= 1 {
		return nil, nil
	}
	keepIdx := 0
	for i, record := range managed {
		if record.Content == target {
			keepIdx = i
			break
		}
	}
	out := make([]sdkrouter.RouterSyncAction, 0, len(managed)-1)
	for i, record := range managed {
		if i == keepIdx {
			continue
		}
		out = append(out, sdkrouter.RouterSyncAction{
			Resource: "dns_record",
			Action:   "delete-candidate",
			Name:     routing.Hostname,
			Detail:   fmt.Sprintf("managed duplicate CNAME -> %s (TTL %d)", record.Content, record.TTL),
		})
	}
	return out, nil
}

func (s *cloudflareSyncer) collectLBDeleteCandidates(ctx context.Context, routing *sdkrouter.RoutingConfig, monitorID, poolID string) ([]sdkrouter.RouterSyncAction, error) {
	serviceTag := managedByTag + " service=" + routing.Service
	out := make([]sdkrouter.RouterSyncAction, 0)

	monitors, err := s.client.listMonitors(ctx)
	if err != nil {
		return nil, err
	}
	for _, monitor := range monitors {
		if monitor.ID == monitorID {
			continue
		}
		if strings.TrimSpace(monitor.Description) != serviceTag {
			continue
		}
		out = append(out, sdkrouter.RouterSyncAction{
			Resource: "lb_monitor",
			Action:   "delete-candidate",
			Name:     monitorName(routing),
			Detail:   fmt.Sprintf("managed monitor id=%s path=%s", monitor.ID, monitor.Path),
		})
	}

	pools, err := s.client.listPools(ctx)
	if err != nil {
		return nil, err
	}
	poolDesc := managedByTag + " service=" + routing.Service + " stage=" + routing.Stage
	for _, pool := range pools {
		if pool.ID == poolID {
			continue
		}
		if strings.TrimSpace(pool.Description) != poolDesc {
			continue
		}
		out = append(out, sdkrouter.RouterSyncAction{
			Resource: "lb_pool",
			Action:   "delete-candidate",
			Name:     poolName(routing),
			Detail:   fmt.Sprintf("managed pool id=%s origins=%d", pool.ID, len(pool.Origins)),
		})
	}

	return out, nil
}

func auditCloudflareAPIToken(token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return fmt.Errorf("router API token is required (set RUNFABRIC_ROUTER_API_TOKEN/CLOUDFLARE_API_TOKEN or *_API_TOKEN_FILE)")
	}
	if trimmed != token || strings.ContainsAny(token, " \t\r\n") {
		return fmt.Errorf("router API token must not include whitespace")
	}
	if strings.Contains(trimmed, "***") {
		return fmt.Errorf("router API token appears redacted; provide the full secret value")
	}
	lowered := strings.ToLower(trimmed)
	for _, marker := range []string{"changeme", "replace_me", "replace-me", "replace-with", "your_token", "placeholder", "example", "dummy", "redacted"} {
		if strings.Contains(lowered, marker) {
			return fmt.Errorf("router API token appears to be a placeholder value")
		}
	}
	if len(trimmed) < 20 {
		return fmt.Errorf("router API token looks too short; use a real provider-issued token")
	}
	return nil
}

func (s *cloudflareSyncer) printSummary(result *sdkrouter.RouterSyncResult) {
	prefix, verb := "", ""
	if s.dryRun {
		prefix, verb = "[DRY RUN] ", "Would "
	}
	created, updated, noops := 0, 0, 0
	for _, a := range result.Actions {
		detail := ""
		if a.Detail != "" {
			detail = " (" + a.Detail + ")"
		}
		switch a.Action {
		case "create":
			created++
			fmt.Fprintf(s.out, "%s%screate %s: %s%s\n", prefix, verb, a.Resource, a.Name, detail)
		case "update":
			updated++
			fmt.Fprintf(s.out, "%s%supdate %s: %s%s\n", prefix, verb, a.Resource, a.Name, detail)
		case "no-op":
			noops++
		}
	}
	if s.dryRun {
		fmt.Fprintf(s.out, "sync plan: %d to create, %d to update, %d unchanged\n", created, updated, noops)
	} else {
		fmt.Fprintf(s.out, "sync complete: %d created, %d updated, %d unchanged\n", created, updated, noops)
	}
}

func poolName(r *sdkrouter.RoutingConfig) string {
	if r.Stage != "" {
		return r.Service + "-" + r.Stage + "-pool"
	}
	return r.Service + "-pool"
}

func monitorName(r *sdkrouter.RoutingConfig) string {
	if r.Stage != "" {
		return r.Service + "-" + r.Stage + "-monitor"
	}
	return r.Service + "-monitor"
}

func lbName(r *sdkrouter.RoutingConfig) string { return r.Hostname }

func steeringPolicy(strategy string) string {
	switch strings.ToLower(strategy) {
	case "latency":
		return "dynamic_latency"
	case "failover":
		return "off"
	default:
		return "random"
	}
}

func toOrigins(endpoints []sdkrouter.RoutingEndpoint) []cloudflareLBOrigin {
	origins := make([]cloudflareLBOrigin, len(endpoints))
	for i, ep := range endpoints {
		w := float64(ep.Weight)
		if w <= 0 {
			w = 1
		}
		origins[i] = cloudflareLBOrigin{Name: ep.Name, Address: ep.URL, Enabled: true, Weight: w}
	}
	return origins
}

func originsEqual(a, b []cloudflareLBOrigin) bool {
	if len(a) != len(b) {
		return false
	}
	type originShape struct {
		address string
		weight  float64
	}
	byName := make(map[string]originShape, len(a))
	for _, o := range a {
		byName[o.Name] = originShape{address: o.Address, weight: o.Weight}
	}
	for _, o := range b {
		item, ok := byName[o.Name]
		if !ok {
			return false
		}
		if item.address != o.Address || item.weight != o.Weight {
			return false
		}
	}
	return true
}

func stripScheme(rawURL string) string {
	if after, ok := strings.CutPrefix(rawURL, "https://"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(rawURL, "http://"); ok {
		return after
	}
	return rawURL
}
