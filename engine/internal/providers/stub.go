// Package providers provides a fallback Provider for matrix providers without a full
// implementation in providers/<name>. Lifecycle actions (doctor, plan, deploy, remove,
// invoke, logs) run with simulated or no-op behavior per the Trigger Capability Matrix.
// See docs/ARCHITECTURE.md for provider layout.

package providers

import (
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/planner"
)

// namedProvider wraps a Provider to expose a different Name(). Used to register
// the same implementation under multiple names (e.g. aws and aws-lambda).
type namedProvider struct {
	name string
	Provider
}

// NewNamedProvider returns a Provider that delegates to p but reports Name() as name.
func NewNamedProvider(name string, p Provider) Provider {
	return &namedProvider{name: name, Provider: p}
}

func (n *namedProvider) Name() string {
	return n.name
}

// StubProvider implements Provider for a given provider name with simulated
// behavior so doctor/plan/deploy/remove/invoke/logs do not return "provider not found".
// Deploy is simulated (writes no real resources); triggers are validated against
// the Trigger Capability Matrix.
type StubProvider struct {
	name string
}

// NewStubProvider returns a Provider stub for the given provider name (e.g. gcp-functions).
// Name must exist in planner.ProviderCapabilities.
func NewStubProvider(name string) *StubProvider {
	return &StubProvider{name: name}
}

func (s *StubProvider) Name() string {
	return s.name
}

func (s *StubProvider) Doctor(cfg *config.Config, stage string) (*DoctorResult, error) {
	triggers := planner.SupportedTriggers(s.name)
	return &DoctorResult{
		Provider: s.name,
		Checks: []string{
			fmt.Sprintf("Stub provider %q registered", s.name),
			fmt.Sprintf("Supported triggers: %v", triggers),
			"No real credentials checked; implement full Doctor for production use.",
		},
	}, nil
}

func (s *StubProvider) Plan(cfg *config.Config, stage, root string) (*PlanResult, error) {
	warnings := planner.ValidateTriggersForProvider(cfg, s.name)
	triggers := planner.ExtractTriggers(cfg)
	actions := make([]planner.PlanAction, 0, len(triggers)*2)
	for _, ft := range triggers {
		for _, spec := range ft.Specs {
			if !planner.SupportsTrigger(s.name, spec.Kind) {
				continue
			}
			actions = append(actions, planner.PlanAction{
				ID:          fmt.Sprintf("trigger:%s:%s:%s", spec.Kind, ft.Function, spec.Kind),
				Type:        planner.ActionCreate,
				Resource:    planner.ResourceTypeForTrigger(spec.Kind),
				Name:        ft.Function + ":" + spec.Kind,
				Description: fmt.Sprintf("Create %s trigger for %s", spec.Kind, ft.Function),
			})
		}
	}
	return &PlanResult{
		Provider: s.name,
		Plan:     &planner.Plan{Provider: s.name, Service: cfg.Service, Stage: stage, Actions: actions},
		Warnings: warnings,
	}, nil
}

func (s *StubProvider) Deploy(cfg *config.Config, stage, root string) (*DeployResult, error) {
	// Simulated deploy: no real resources created; return a result so receipt can be written if needed
	artifacts := make([]Artifact, 0, len(cfg.Functions))
	for name, fn := range cfg.Functions {
		runtime := fn.Runtime
		if runtime == "" {
			runtime = cfg.Provider.Runtime
		}
		artifacts = append(artifacts, Artifact{
			Function: name,
			Runtime:  runtime,
		})
	}
	return &DeployResult{
		Provider:     s.name,
		DeploymentID: "stub-" + stage,
		Outputs:      map[string]string{"message": "simulated deploy; no real resources created"},
		Artifacts:    artifacts,
		Metadata:     map[string]string{"stub": "true"},
	}, nil
}

func (s *StubProvider) Remove(cfg *config.Config, stage, root string) (*RemoveResult, error) {
	return &RemoveResult{Provider: s.name, Removed: true}, nil
}

func (s *StubProvider) Invoke(cfg *config.Config, stage, function string, payload []byte) (*InvokeResult, error) {
	return &InvokeResult{
		Provider: s.name,
		Function: function,
		Output:   fmt.Sprintf("Invoke not implemented for provider %q; add real Invoke in provider adapter", s.name),
	}, nil
}

func (s *StubProvider) Logs(cfg *config.Config, stage, function string) (*LogsResult, error) {
	return &LogsResult{
		Provider: s.name,
		Function: function,
		Lines:    []string{fmt.Sprintf("Logs not implemented for provider %q; add real Logs in provider adapter", s.name)},
	}, nil
}
