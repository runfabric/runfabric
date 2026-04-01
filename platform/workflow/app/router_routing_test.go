package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func TestGenerateRouterRoutingConfig_DeterministicContractV1(t *testing.T) {
	healthy := true
	fabricState := &state.RunFabricState{
		Service: "svc",
		Stage:   "dev",
		Endpoints: []state.RunFabricEndpoint{
			{Provider: "z-provider", URL: "https://z.example.com", Healthy: &healthy},
			{Provider: "a-provider", URL: "https://a.example.com", Healthy: &healthy},
		},
	}
	cfg := &config.Config{
		Service: "my-service",
		Fabric: &config.FabricConfig{
			Routing: "latency",
			HealthCheck: &config.HealthCheckConfig{
				URL: "https://health.example.com/readyz",
			},
		},
	}

	out := GenerateRouterRoutingConfig(fabricState, cfg, "staging")
	if out == nil {
		t.Fatal("expected non-nil routing config")
	}
	if out.Contract != "runfabric.fabric.routing.v1" {
		t.Fatalf("unexpected contract: %q", out.Contract)
	}
	if out.Service != "my-service" {
		t.Fatalf("unexpected service: %q", out.Service)
	}
	if out.Stage != "staging" {
		t.Fatalf("unexpected stage: %q", out.Stage)
	}
	if out.Hostname != "my-service" {
		t.Fatalf("unexpected hostname: %q", out.Hostname)
	}
	if out.Strategy != "latency" {
		t.Fatalf("unexpected strategy: %q", out.Strategy)
	}
	if out.HealthPath != "/readyz" {
		t.Fatalf("unexpected healthPath: %q", out.HealthPath)
	}
	if out.TTL != 60 {
		t.Fatalf("unexpected ttl for latency strategy: %d", out.TTL)
	}
	if len(out.Endpoints) != 2 {
		t.Fatalf("unexpected endpoint count: %d", len(out.Endpoints))
	}
	if out.Endpoints[0].Name != "a-provider" || out.Endpoints[1].Name != "z-provider" {
		t.Fatalf("endpoints are not sorted deterministically: %+v", out.Endpoints)
	}
	if out.DNS == nil || len(out.DNS.Records) == 0 {
		t.Fatal("expected dns records")
	}
	if out.DNS.Records[0].TTL != out.TTL {
		t.Fatalf("expected dns record ttl %d, got %d", out.TTL, out.DNS.Records[0].TTL)
	}
}
