// Package conformance is the single provider-contract conformance suite.
//
// Instead of per-provider ad-hoc test folders, every provider is exercised
// through one parameterized lifecycle — validate → plan → deploy → invoke →
// logs → remove — and checked against the invariants any RunFabric provider
// must satisfy. Passing this suite is the entry bar for a support tier
// (see docs/PROVIDER_TIERS.md): adding a provider becomes "make it pass the
// conformance suite", and breadth becomes a checklist rather than a guess.
//
// Usage from a provider's own tests:
//
//	func TestMyProviderConformance(t *testing.T) {
//	    conformance.RunProviderConformance(t, NewMyProvider(),
//	        conformance.SampleConfig("my-provider"), t.TempDir(), "dev")
//	}
package conformance

import (
	"context"
	"fmt"
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// SampleConfig returns a minimal valid config for conformance runs: one service
// with a single HTTP function targeting the named provider.
func SampleConfig(providerName string) *config.Config {
	return &config.Config{
		Service:  "conformance-svc",
		Provider: config.ProviderConfig{Name: providerName, Runtime: "nodejs20.x"},
		Functions: map[string]config.FunctionConfig{
			ConformanceFunction: {Handler: "index.handler", Runtime: "nodejs20.x"},
		},
	}
}

// ConformanceFunction is the logical function name the suite deploys/invokes.
const ConformanceFunction = "hello"

// CheckProviderConformance runs the full provider contract and returns a list of
// human-readable violations (empty when the provider is compliant). It performs
// no test assertions itself, so it is usable both to gate a provider and to
// unit-test the suite (a deliberately broken provider must produce violations).
func CheckProviderConformance(ctx context.Context, p providers.ProviderPlugin, cfg *config.Config, root, stage string) []string {
	if stage == "" {
		stage = "dev"
	}
	var v []string
	add := func(format string, args ...any) { v = append(v, fmt.Sprintf(format, args...)) }

	if p == nil {
		return []string{"provider is nil"}
	}
	if p.Meta().Name == "" {
		add("Meta().Name must be non-empty")
	}

	if err := p.ValidateConfig(ctx, providers.ValidateConfigRequest{Config: cfg, Stage: stage}); err != nil {
		add("ValidateConfig on a valid config returned error: %v", err)
	}

	if res, err := p.Plan(ctx, providers.PlanRequest{Config: cfg, Stage: stage, Root: root}); err != nil {
		add("Plan returned error: %v", err)
	} else if res == nil {
		add("Plan returned a nil result")
	}

	if res, err := p.Deploy(ctx, providers.DeployRequest{Config: cfg, Stage: stage, Root: root}); err != nil {
		add("Deploy returned error: %v", err)
	} else if res == nil {
		add("Deploy returned a nil result")
	} else {
		if res.Provider == "" {
			add("DeployResult.Provider must be set")
		}
		if res.DeploymentID == "" {
			add("DeployResult.DeploymentID must be set so the deploy is addressable")
		}
	}

	if res, err := p.Invoke(ctx, providers.InvokeRequest{Config: cfg, Stage: stage, Function: ConformanceFunction, Payload: []byte("{}")}); err != nil {
		add("Invoke returned error: %v", err)
	} else if res == nil {
		add("Invoke returned a nil result")
	} else if res.Function != ConformanceFunction {
		add("InvokeResult.Function = %q, want %q (must echo the invoked function)", res.Function, ConformanceFunction)
	}

	if res, err := p.Logs(ctx, providers.LogsRequest{Config: cfg, Stage: stage, Function: ConformanceFunction}); err != nil {
		add("Logs returned error: %v", err)
	} else if res == nil {
		add("Logs returned a nil result")
	}

	if res, err := p.Remove(ctx, providers.RemoveRequest{Config: cfg, Stage: stage, Root: root}); err != nil {
		add("Remove returned error: %v", err)
	} else if res == nil {
		add("Remove returned a nil result")
	} else if !res.Removed {
		add("RemoveResult.Removed must be true after a successful Remove")
	}

	return v
}

// RunProviderConformance runs the suite and fails t with one error per contract
// violation. It is the entry point provider tests call.
func RunProviderConformance(t *testing.T, p providers.ProviderPlugin, cfg *config.Config, root, stage string) {
	t.Helper()
	for _, msg := range CheckProviderConformance(context.Background(), p, cfg, root, stage) {
		t.Errorf("provider conformance: %s", msg)
	}
}
