package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
)

func TestApplyCanaryWeights(t *testing.T) {
	routing := &RouterRoutingConfig{
		Endpoints: []RouterRoutingEndpoint{
			{Name: "aws-us", Weight: 100},
			{Name: "gcp-eu", Weight: 100},
			{Name: "azure-apac", Weight: 100},
		},
	}
	ok := ApplyCanaryWeights(routing, "gcp-eu", 20)
	if !ok {
		t.Fatal("expected canary shift to succeed")
	}
	sum := 0
	canaryWeight := 0
	for _, ep := range routing.Endpoints {
		sum += ep.Weight
		if ep.Name == "gcp-eu" {
			canaryWeight = ep.Weight
		}
		if ep.Weight < 1 {
			t.Fatalf("expected positive weight for %s, got %d", ep.Name, ep.Weight)
		}
	}
	if sum != 100 {
		t.Fatalf("expected total weight 100, got %d", sum)
	}
	if canaryWeight != 20 {
		t.Fatalf("expected canary weight 20, got %d", canaryWeight)
	}
}

func TestApplyCanaryWeights_UnknownProvider(t *testing.T) {
	routing := &RouterRoutingConfig{
		Endpoints: []RouterRoutingEndpoint{{Name: "aws-us", Weight: 100}, {Name: "gcp-eu", Weight: 100}},
	}
	if ApplyCanaryWeights(routing, "missing", 15) {
		t.Fatal("expected unknown canary provider to return false")
	}
}

func TestGenerateRouterRoutingConfig_AppliesQualityScoring(t *testing.T) {
	healthy := true
	unhealthy := false
	fabricState := &state.RunFabricState{
		Service: "svc",
		Stage:   "dev",
		Endpoints: []state.RunFabricEndpoint{
			{Provider: "fast", URL: "https://fast.example.com", Healthy: &healthy},
			{Provider: "slow", URL: "https://slow.example.com", Healthy: &unhealthy},
		},
	}
	cfg := &config.Config{
		Service: "svc",
		Fabric:  &config.FabricConfig{Routing: "round-robin"},
		Extensions: map[string]any{
			"router": map[string]any{
				"qualityScoring": map[string]any{
					"enabled":                 true,
					"unhealthyPenaltyPercent": 90,
					"providerMultiplier": map[string]any{
						"fast": 150,
					},
				},
			},
		},
	}
	out := GenerateRouterRoutingConfig(fabricState, cfg, "dev")
	if out == nil || len(out.Endpoints) != 2 {
		t.Fatal("expected two endpoints")
	}
	var fastWeight, slowWeight int
	for _, ep := range out.Endpoints {
		if ep.Name == "fast" {
			fastWeight = ep.Weight
		}
		if ep.Name == "slow" {
			slowWeight = ep.Weight
		}
	}
	if fastWeight <= slowWeight {
		t.Fatalf("expected quality scoring to prefer fast endpoint: fast=%d slow=%d", fastWeight, slowWeight)
	}
	if fastWeight+slowWeight != 100 {
		t.Fatalf("expected normalized weights to sum 100, got %d", fastWeight+slowWeight)
	}
}

func TestSimulateRouterRouting_FailoverWithPrimaryDown(t *testing.T) {
	healthy := true
	routing := &RouterRoutingConfig{
		Strategy: "failover",
		Endpoints: []RouterRoutingEndpoint{
			{Name: "primary", Priority: 1, Weight: 100, Healthy: &healthy},
			{Name: "backup", Priority: 2, Weight: 100, Healthy: &healthy},
		},
	}
	result := SimulateRouterRouting(routing, 50, []string{"primary"})
	if !result.Available {
		t.Fatal("expected backup to remain available")
	}
	if result.Selected != "backup" {
		t.Fatalf("expected backup to be selected, got %q", result.Selected)
	}
	if result.Distribution["backup"] != 50 {
		t.Fatalf("expected all requests on backup, got %#v", result.Distribution)
	}
}

func TestVerifyRouterFailover(t *testing.T) {
	healthy := true
	routing := &RouterRoutingConfig{
		Strategy: "round-robin",
		Endpoints: []RouterRoutingEndpoint{
			{Name: "a", Weight: 60, Healthy: &healthy},
			{Name: "b", Weight: 40, Healthy: &healthy},
		},
	}
	report := VerifyRouterFailover(routing, 100)
	if !report.Pass {
		t.Fatalf("expected failover verification to pass, got %#v", report)
	}
	if len(report.Scenarios) != 3 {
		t.Fatalf("expected 3 scenarios (a-down, b-down, all-down), got %d", len(report.Scenarios))
	}
}
