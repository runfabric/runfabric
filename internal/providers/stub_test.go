package providers

import (
	"testing"

	"github.com/runfabric/runfabric/internal/config"
)

func TestStubProvider_Doctor(t *testing.T) {
	p := NewStubProvider("gcp-functions")
	if p.Name() != "gcp-functions" {
		t.Errorf("Name() = %q, want gcp-functions", p.Name())
	}
	cfg := &config.Config{
		Service:  "test",
		Provider: config.ProviderConfig{Name: "gcp-functions", Runtime: "nodejs20"},
		Functions: map[string]config.FunctionConfig{"fn": {Handler: "index.handler"}},
	}
	res, err := p.Doctor(cfg, "dev")
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if res.Provider != "gcp-functions" {
		t.Errorf("Provider = %q", res.Provider)
	}
	if len(res.Checks) == 0 {
		t.Error("expected at least one check")
	}
}

func TestStubProvider_Plan(t *testing.T) {
	p := NewStubProvider("kubernetes")
	cfg := &config.Config{
		Service:  "test",
		Provider: config.ProviderConfig{Name: "kubernetes", Runtime: "nodejs20"},
		Functions: map[string]config.FunctionConfig{
			"fn": {Handler: "index.handler", Events: []config.EventConfig{{HTTP: &config.HTTPEvent{Path: "/", Method: "GET"}}}},
		},
	}
	res, err := p.Plan(cfg, "dev", "/tmp")
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if res.Provider != "kubernetes" {
		t.Errorf("Provider = %q", res.Provider)
	}
	if res.Plan == nil {
		t.Fatal("Plan is nil")
	}
}

func TestStubProvider_Deploy(t *testing.T) {
	p := NewStubProvider("netlify")
	cfg := &config.Config{
		Service:  "test",
		Provider: config.ProviderConfig{Name: "netlify", Runtime: "nodejs20"},
		Functions: map[string]config.FunctionConfig{"fn": {Handler: "index.handler"}},
	}
	res, err := p.Deploy(cfg, "dev", "/tmp")
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if res.Provider != "netlify" {
		t.Errorf("Provider = %q", res.Provider)
	}
	if res.DeploymentID == "" {
		t.Error("DeploymentID empty")
	}
	if len(res.Artifacts) != 1 || res.Artifacts[0].Function != "fn" {
		t.Errorf("Artifacts = %v", res.Artifacts)
	}
}

func TestStubProvider_Remove_Invoke_Logs(t *testing.T) {
	p := NewStubProvider("vercel")
	cfg := &config.Config{
		Service:  "test",
		Provider: config.ProviderConfig{Name: "vercel", Runtime: "nodejs20"},
		Functions: map[string]config.FunctionConfig{"fn": {Handler: "index.handler"}},
	}
	rem, err := p.Remove(cfg, "dev", "/tmp")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !rem.Removed {
		t.Error("Removed should be true")
	}
	inv, err := p.Invoke(cfg, "dev", "fn", nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if inv.Output == "" {
		t.Error("Invoke output empty")
	}
	logs, err := p.Logs(cfg, "dev", "fn")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(logs.Lines) == 0 {
		t.Error("Logs lines empty")
	}
}
