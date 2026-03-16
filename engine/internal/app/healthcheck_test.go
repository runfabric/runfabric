package app

import (
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
)

func TestRunPostDeployHealthCheck_DisabledOrNoURL(t *testing.T) {
	cfg := &config.Config{
		Service:   "svc",
		Provider:  config.ProviderConfig{Name: "aws", Runtime: "nodejs"},
		Functions: map[string]config.FunctionConfig{"api": {Handler: "h"}},
	}
	// Health check not configured: no error
	if err := runPostDeployHealthCheck(cfg, nil, "/tmp", "dev", "", false); err != nil {
		t.Errorf("expected nil when health check not configured, got %v", err)
	}
	cfg.Deploy = &config.DeployConfig{}
	if err := runPostDeployHealthCheck(cfg, nil, "/tmp", "dev", "", false); err != nil {
		t.Errorf("expected nil when healthCheck nil, got %v", err)
	}
	cfg.Deploy.HealthCheck = &config.HealthCheckConfig{Enabled: false}
	if err := runPostDeployHealthCheck(cfg, nil, "/tmp", "dev", "", false); err != nil {
		t.Errorf("expected nil when health check disabled, got %v", err)
	}
	cfg.Deploy.HealthCheck.Enabled = true
	// No URL and result has no outputs: skip, no error
	if err := runPostDeployHealthCheck(cfg, nil, "/tmp", "dev", "", false); err != nil {
		t.Errorf("expected nil when no URL and no result outputs, got %v", err)
	}
}
