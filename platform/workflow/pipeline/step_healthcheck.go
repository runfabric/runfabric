package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// HealthCheckStep performs an HTTP GET against the deployed URL after deploy.
// Skipped when cfg.Deploy.HealthCheck is not enabled or no URL is available.
type HealthCheckStep struct {
	RollbackOnFailure bool
	OnRollback        func(ctx context.Context) error // called when check fails and rollback is enabled
}

func (s HealthCheckStep) Name() string { return "health-check" }

func (s HealthCheckStep) Run(ctx context.Context, sc *StepContext) error {
	if sc.Config.Deploy == nil || sc.Config.Deploy.HealthCheck == nil || !sc.Config.Deploy.HealthCheck.Enabled {
		return nil
	}

	url := strings.TrimSpace(sc.Config.Deploy.HealthCheck.URL)
	if url == "" && sc.DeployResult != nil {
		for _, k := range []string{"ServiceURL", "url", "ApiUrl", "endpoint"} {
			if v, ok := sc.DeployResult.Outputs[k]; ok && strings.TrimSpace(v) != "" {
				url = strings.TrimSpace(v)
				break
			}
		}
	}
	if url == "" {
		return nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		if s.RollbackOnFailure && s.OnRollback != nil {
			_ = s.OnRollback(ctx)
		}
		return fmt.Errorf("GET %s: %w", url, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if s.RollbackOnFailure && s.OnRollback != nil {
			_ = s.OnRollback(ctx)
		}
		return fmt.Errorf("GET %s returned %d", url, resp.StatusCode)
	}
	return nil
}
