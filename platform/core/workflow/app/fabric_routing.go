package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

// FabricRoutingConfig holds routing strategy configuration generated from fabric endpoints.
type FabricRoutingConfig struct {
	Strategy     string                     `json:"strategy"`             // "failover" | "latency" | "round-robin"
	HealthPath   string                     `json:"healthPath,omitempty"` // path for health checks (e.g. /health)
	Endpoints    []FabricRoutingEndpoint    `json:"endpoints"`
	DNS          *DNSConfiguration          `json:"dns,omitempty"`          // generated DNS configuration
	LoadBalancer *LoadBalancerConfiguration `json:"loadBalancer,omitempty"` // generated LB configuration
}

// FabricRoutingEndpoint represents a single endpoint in the routing configuration.
type FabricRoutingEndpoint struct {
	Name     string `json:"name"`     // provider key
	URL      string `json:"url"`      // endpoint URL
	Healthy  *bool  `json:"healthy"`  // health status
	Priority int    `json:"priority"` // used for failover (lower = higher priority)
	Weight   int    `json:"weight"`   // used for load balancing
	Region   string `json:"region,omitempty"`
}

// DNSConfiguration provides DNS record hints for the chosen routing strategy.
type DNSConfiguration struct {
	Type    string      `json:"type"`    // "route53" | "cloudflare" | "ns1" | "generic"
	Records []DNSRecord `json:"records"` // DNS records to configure
}

// DNSRecord represents a single DNS record.
type DNSRecord struct {
	Type     string   `json:"type"`               // "A" | "CNAME" | "NS" | "SRV" etc.
	Name     string   `json:"name"`               // record name
	TTL      int      `json:"ttl,omitempty"`      // time to live in seconds
	Values   []string `json:"values,omitempty"`   // record values
	SetID    string   `json:"setId,omitempty"`    // for weighted/geolocation routing
	Weight   int      `json:"weight,omitempty"`   // for weighted routing
	Region   string   `json:"region,omitempty"`   // for geolocation/latency routing
	Priority int      `json:"priority,omitempty"` // for MX/SRV or failover
}

// LoadBalancerConfiguration provides load balancer configuration hints.
type LoadBalancerConfiguration struct {
	Type        string                    `json:"type"` // "aws-alb" | "aws-nlb" | "cloudflare-lb" | "generic"
	Upstreams   []LoadBalancerUpstream    `json:"upstreams"`
	HealthCheck *HealthCheckConfiguration `json:"healthCheck,omitempty"`
	Strategy    string                    `json:"strategy"` // "round-robin" | "least-conn" | "geo-proximity" | "latency"
}

// LoadBalancerUpstream represents an upstream in the load balancer configuration.
type LoadBalancerUpstream struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Weight   int    `json:"weight,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

// HealthCheckConfiguration for load balancer health checks.
type HealthCheckConfiguration struct {
	Path     string `json:"path,omitempty"`     // e.g. /health
	Interval int    `json:"interval,omitempty"` // seconds
	Timeout  int    `json:"timeout,omitempty"`  // seconds
}

// GenerateFabricRoutingConfig generates routing configuration based on fabric state and routing strategy.
func GenerateFabricRoutingConfig(fabricState *state.FabricState, cfg *config.Config) *FabricRoutingConfig {
	if fabricState == nil || cfg == nil || cfg.Fabric == nil {
		return nil
	}

	strategy := cfg.Fabric.Routing
	if strategy == "" {
		strategy = "round-robin" // default strategy
	}

	healthPath := "/health"
	if cfg.Fabric.HealthCheck != nil && cfg.Fabric.HealthCheck.URL != "" {
		if parsed, err := url.Parse(cfg.Fabric.HealthCheck.URL); err == nil && parsed.Path != "" {
			healthPath = parsed.Path
		}
	}

	routingCfg := &FabricRoutingConfig{
		Strategy:   strategy,
		HealthPath: healthPath,
		Endpoints:  make([]FabricRoutingEndpoint, 0, len(fabricState.Endpoints)),
	}

	// Convert fabric endpoints to routing endpoints
	for i, endpoint := range fabricState.Endpoints {
		rEndpoint := FabricRoutingEndpoint{
			Name:     endpoint.Provider,
			URL:      endpoint.URL,
			Healthy:  endpoint.Healthy,
			Priority: i + 1, // failover priority
			Weight:   100,   // equal weight by default
		}
		routingCfg.Endpoints = append(routingCfg.Endpoints, rEndpoint)
	}

	// Generate strategy-specific configurations
	switch strategy {
	case "failover":
		routingCfg.DNS = generateFailoverDNS(cfg.Service, routingCfg.Endpoints)
		routingCfg.LoadBalancer = generateFailoverLoadBalancer(routingCfg.Endpoints)
	case "latency":
		routingCfg.DNS = generateLatencyDNS(cfg.Service, routingCfg.Endpoints)
		routingCfg.LoadBalancer = generateLatencyLoadBalancer(routingCfg.Endpoints)
	case "round-robin":
		fallthrough
	default:
		routingCfg.DNS = generateRoundRobinDNS(cfg.Service, routingCfg.Endpoints)
		routingCfg.LoadBalancer = generateRoundRobinLoadBalancer(routingCfg.Endpoints)
	}

	return routingCfg
}

// generateFailoverDNS generates DNS configuration for failover routing.
func generateFailoverDNS(service string, endpoints []FabricRoutingEndpoint) *DNSConfiguration {
	records := make([]DNSRecord, 0)

	// Create failover DNS records with priority
	for i, endpoint := range endpoints {
		priority := i + 1 // ascending priority (lower = primary)
		// Extract domain from URL
		domain := extractDomainFromURL(endpoint.URL)

		records = append(records, DNSRecord{
			Type:     "A",
			Name:     service,
			TTL:      300,
			Values:   []string{domain},
			Priority: priority,
		})
	}

	return &DNSConfiguration{
		Type:    "generic",
		Records: records,
	}
}

// generateLatencyDNS generates DNS configuration for latency-based routing.
func generateLatencyDNS(service string, endpoints []FabricRoutingEndpoint) *DNSConfiguration {
	records := make([]DNSRecord, 0)

	// Create latency-based DNS records (requires Route53 or similar)
	for _, endpoint := range endpoints {
		domain := extractDomainFromURL(endpoint.URL)
		region := endpoint.Region
		if region == "" {
			region = "us-east-1" // default
		}

		records = append(records, DNSRecord{
			Type:   "A",
			Name:   service,
			TTL:    60,
			Values: []string{domain},
			Region: region,
			SetID:  endpoint.Name,
		})
	}

	return &DNSConfiguration{
		Type:    "route53",
		Records: records,
	}
}

// generateRoundRobinDNS generates DNS configuration for round-robin routing.
func generateRoundRobinDNS(service string, endpoints []FabricRoutingEndpoint) *DNSConfiguration {
	records := make([]DNSRecord, 0)

	// Create round-robin DNS records with equal weights
	for _, endpoint := range endpoints {
		domain := extractDomainFromURL(endpoint.URL)

		records = append(records, DNSRecord{
			Type:   "A",
			Name:   service,
			TTL:    300,
			Values: []string{domain},
			Weight: 100,
			SetID:  endpoint.Name,
		})
	}

	return &DNSConfiguration{
		Type:    "generic",
		Records: records,
	}
}

// generateFailoverLoadBalancer generates load balancer configuration for failover.
func generateFailoverLoadBalancer(endpoints []FabricRoutingEndpoint) *LoadBalancerConfiguration {
	upstreams := make([]LoadBalancerUpstream, 0, len(endpoints))

	for i, endpoint := range endpoints {
		upstreams = append(upstreams, LoadBalancerUpstream{
			Name:     endpoint.Name,
			URL:      endpoint.URL,
			Priority: i + 1,
		})
	}

	return &LoadBalancerConfiguration{
		Type:      "generic",
		Upstreams: upstreams,
		Strategy:  "failover",
		HealthCheck: &HealthCheckConfiguration{
			Path:     "/health",
			Interval: 30,
			Timeout:  5,
		},
	}
}

// generateLatencyLoadBalancer generates load balancer configuration for latency-based routing.
func generateLatencyLoadBalancer(endpoints []FabricRoutingEndpoint) *LoadBalancerConfiguration {
	upstreams := make([]LoadBalancerUpstream, 0, len(endpoints))

	for _, endpoint := range endpoints {
		upstreams = append(upstreams, LoadBalancerUpstream{
			Name:   endpoint.Name,
			URL:    endpoint.URL,
			Weight: 100, // equal weight for latency-based
		})
	}

	return &LoadBalancerConfiguration{
		Type:      "generic",
		Upstreams: upstreams,
		Strategy:  "latency",
		HealthCheck: &HealthCheckConfiguration{
			Path:     "/health",
			Interval: 10,
			Timeout:  3,
		},
	}
}

// generateRoundRobinLoadBalancer generates load balancer configuration for round-robin.
func generateRoundRobinLoadBalancer(endpoints []FabricRoutingEndpoint) *LoadBalancerConfiguration {
	upstreams := make([]LoadBalancerUpstream, 0, len(endpoints))

	for _, endpoint := range endpoints {
		upstreams = append(upstreams, LoadBalancerUpstream{
			Name:   endpoint.Name,
			URL:    endpoint.URL,
			Weight: 100,
		})
	}

	return &LoadBalancerConfiguration{
		Type:      "generic",
		Upstreams: upstreams,
		Strategy:  "round-robin",
		HealthCheck: &HealthCheckConfiguration{
			Path:     "/health",
			Interval: 30,
			Timeout:  5,
		},
	}
}

// extractDomainFromURL extracts the domain from a URL.
func extractDomainFromURL(urlStr string) string {
	// Remove https:// or http://
	url := strings.TrimPrefix(strings.TrimPrefix(urlStr, "https://"), "http://")
	// Remove path and query string
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}
	return url
}

// FormatFabricRoutingGuide returns a human-readable guide for configuring DNS/LB based on the routing strategy.
func FormatFabricRoutingGuide(routingCfg *FabricRoutingConfig) string {
	if routingCfg == nil {
		return ""
	}

	var guide strings.Builder
	guide.WriteString(fmt.Sprintf("Fabric Routing Configuration Guide (%s)\n", strings.ToUpper(routingCfg.Strategy)))
	guide.WriteString("=====================================\n\n")

	guide.WriteString("Endpoints:\n")
	for _, ep := range routingCfg.Endpoints {
		healthStatus := "unknown"
		if ep.Healthy != nil {
			if *ep.Healthy {
				healthStatus = "healthy"
			} else {
				healthStatus = "unhealthy"
			}
		}
		guide.WriteString(fmt.Sprintf("  %s: %s (%s)\n", ep.Name, ep.URL, healthStatus))
	}
	guide.WriteString("\n")

	switch routingCfg.Strategy {
	case "failover":
		guide.WriteString("Failover Strategy:\n")
		guide.WriteString("  - Primary endpoint: " + routingCfg.Endpoints[0].Name + "\n")
		if len(routingCfg.Endpoints) > 1 {
			guide.WriteString("  - Failover to: " + routingCfg.Endpoints[1].Name + "\n")
		}
		guide.WriteString("  Configure your DNS or load balancer to point to the primary endpoint.\n")
		guide.WriteString("  If the primary becomes unhealthy, manually switch to a failover endpoint.\n")

	case "latency":
		guide.WriteString("Latency-based Routing:\n")
		guide.WriteString("  - Route requests based on lowest latency to each endpoint.\n")
		guide.WriteString("  Use Route53 (AWS), Cloudflare, or NS1 with latency-based routing policies.\n")
		guide.WriteString("  Each endpoint should have a unique domain in a different region.\n")

	case "round-robin":
		guide.WriteString("Round-robin Distribution:\n")
		guide.WriteString("  - Distribute requests equally across all endpoints.\n")
		guide.WriteString("  Configure your DNS with multiple A records or use a load balancer.\n")
		guide.WriteString("  All endpoints receive equal traffic weight (100).\n")

	default:
		guide.WriteString("Strategy: " + routingCfg.Strategy + "\n")
		guide.WriteString("Define custom routing logic in your DNS provider or load balancer.\n")
	}

	guide.WriteString("\nDNS Configuration:\n")
	if routingCfg.DNS != nil {
		guide.WriteString(fmt.Sprintf("  Type: %s\n", routingCfg.DNS.Type))
		guide.WriteString(fmt.Sprintf("  Records: %d\n", len(routingCfg.DNS.Records)))
		for _, record := range routingCfg.DNS.Records {
			if record.Priority > 0 {
				guide.WriteString(fmt.Sprintf("    - %s (%s) priority=%d\n", record.Name, record.Type, record.Priority))
			} else if record.SetID != "" {
				guide.WriteString(fmt.Sprintf("    - %s (%s) setId=%s\n", record.Name, record.Type, record.SetID))
			} else {
				guide.WriteString(fmt.Sprintf("    - %s (%s)\n", record.Name, record.Type))
			}
		}
	}

	return guide.String()
}
