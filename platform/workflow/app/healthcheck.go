package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// runPostDeployHealthCheck runs an optional HTTP GET to the deployed URL. If cfg.Deploy.HealthCheck is enabled
// and URL is set or can be taken from deploy result outputs, performs the check. On non-2xx response and
// rollbackOnFailure, calls Remove and returns an error.
func runPostDeployHealthCheck(cfg *config.Config, result any, configPath, stage, providerOverride string, rollbackOnFailure bool) error {
	if cfg.Deploy == nil || cfg.Deploy.HealthCheck == nil || !cfg.Deploy.HealthCheck.Enabled {
		return nil
	}
	url := strings.TrimSpace(cfg.Deploy.HealthCheck.URL)
	if url == "" {
		// Use URL from deploy result outputs (e.g. ServiceURL, url, ApiUrl).
		if dr, ok := result.(*providers.DeployResult); ok && dr != nil && len(dr.Outputs) > 0 {
			for _, k := range []string{"ServiceURL", "url", "ApiUrl", "endpoint"} {
				if v, ok := dr.Outputs[k]; ok && strings.TrimSpace(v) != "" {
					url = strings.TrimSpace(v)
					break
				}
			}
		}
	}
	if url == "" {
		return nil // No URL to check; skip
	}
	// Ensure scheme for GET
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		if rollbackOnFailure {
			_, _ = Remove(configPath, stage, providerOverride)
		}
		return fmt.Errorf("health check GET %s: %w", url, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if rollbackOnFailure {
			_, _ = Remove(configPath, stage, providerOverride)
		}
		return fmt.Errorf("health check failed: GET %s returned %d", url, resp.StatusCode)
	}
	return nil
}
