package conformance

import (
	"context"
	"strings"
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
)

// referenceProvider is a minimal, fully-compliant ProviderPlugin. It doubles as
// executable documentation of the contract the conformance suite enforces: a
// new provider that mirrors these return shapes passes the suite.
type referenceProvider struct{}

func (referenceProvider) Meta() providers.ProviderMeta {
	return providers.ProviderMeta{Name: "reference", Version: "0.0.0"}
}
func (referenceProvider) ValidateConfig(context.Context, providers.ValidateConfigRequest) error {
	return nil
}
func (referenceProvider) Doctor(context.Context, providers.DoctorRequest) (*providers.DoctorResult, error) {
	return &providers.DoctorResult{Provider: "reference"}, nil
}
func (referenceProvider) Plan(context.Context, providers.PlanRequest) (*providers.PlanResult, error) {
	return &providers.PlanResult{Provider: "reference"}, nil
}
func (referenceProvider) Deploy(_ context.Context, _ providers.DeployRequest) (*providers.DeployResult, error) {
	return &providers.DeployResult{Provider: "reference", DeploymentID: "deploy-1", Outputs: map[string]string{}}, nil
}
func (referenceProvider) Remove(context.Context, providers.RemoveRequest) (*providers.RemoveResult, error) {
	return &providers.RemoveResult{Provider: "reference", Removed: true}, nil
}
func (referenceProvider) Invoke(_ context.Context, req providers.InvokeRequest) (*providers.InvokeResult, error) {
	return &providers.InvokeResult{Provider: "reference", Function: req.Function, Output: "ok"}, nil
}
func (referenceProvider) Logs(_ context.Context, req providers.LogsRequest) (*providers.LogsResult, error) {
	return &providers.LogsResult{Provider: "reference", Function: req.Function, Lines: []string{"log line"}}, nil
}

var _ providers.ProviderPlugin = referenceProvider{}

// TestReferenceProviderPassesConformance proves a compliant provider passes.
func TestReferenceProviderPassesConformance(t *testing.T) {
	RunProviderConformance(t, referenceProvider{}, SampleConfig("reference"), t.TempDir(), "dev")
}

// brokenProvider violates several contract points: empty Meta name, nil Deploy
// result, wrong Invoke echo, and Removed=false.
type brokenProvider struct{ referenceProvider }

func (brokenProvider) Meta() providers.ProviderMeta { return providers.ProviderMeta{Name: ""} }
func (brokenProvider) Deploy(context.Context, providers.DeployRequest) (*providers.DeployResult, error) {
	return nil, nil
}
func (brokenProvider) Invoke(context.Context, providers.InvokeRequest) (*providers.InvokeResult, error) {
	return &providers.InvokeResult{Provider: "broken", Function: "wrong"}, nil
}
func (brokenProvider) Remove(context.Context, providers.RemoveRequest) (*providers.RemoveResult, error) {
	return &providers.RemoveResult{Provider: "broken", Removed: false}, nil
}

// TestConformanceCatchesViolations proves the suite is not vacuous: a broken
// provider produces violations naming each breach.
func TestConformanceCatchesViolations(t *testing.T) {
	v := CheckProviderConformance(context.Background(), brokenProvider{}, SampleConfig("broken"), t.TempDir(), "dev")
	if len(v) == 0 {
		t.Fatal("expected conformance violations for a broken provider, got none")
	}
	joined := strings.Join(v, "\n")
	for _, want := range []string{"Meta().Name", "Deploy returned a nil result", "InvokeResult.Function", "Removed must be true"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a violation mentioning %q; got:\n%s", want, joined)
		}
	}
}
