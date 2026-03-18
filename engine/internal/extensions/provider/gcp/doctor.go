package gcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

const listLocationsURL = "https://cloudfunctions.googleapis.com/v2/projects/%s/locations"

// Doctor checks GCP credentials and Cloud Functions API access.
func (p *Provider) Doctor(cfg *providers.Config, stage string) (*providers.DoctorResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	if project == "" {
		return &providers.DoctorResult{
			Provider: p.Name(),
			Checks: []string{
				"GCP_PROJECT or GCP_PROJECT_ID is not set (required for deploy/invoke/logs)",
				"See docs/CREDENTIALS.md for GCP setup",
			},
		}, nil
	}

	token := apiutil.Env("GCP_ACCESS_TOKEN")
	if token == "" {
		return &providers.DoctorResult{
			Provider: p.Name(),
			Checks: []string{
				"GCP_ACCESS_TOKEN is not set (e.g. gcloud auth print-access-token or service account key)",
				"Project: " + project,
				"See docs/CREDENTIALS.md for GCP setup",
			},
		}, nil
	}

	region := cfg.Provider.Region
	if region == "" {
		region = "us-central1"
	}

	checks := []string{
		"GCP project: " + project,
		"GCP_ACCESS_TOKEN set",
		"Region (config or default): " + region,
		"Runtime: " + cfg.Provider.Runtime,
	}

	url := fmt.Sprintf(listLocationsURL, project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloud Functions API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		checks = append(checks, "Cloud Functions API access OK")
	} else if resp.StatusCode == 401 {
		checks = append(checks, "Cloud Functions API: token invalid or expired (run gcloud auth print-access-token)")
	} else if resp.StatusCode == 403 {
		checks = append(checks, "Cloud Functions API: permission denied (check project and roles)")
	} else {
		checks = append(checks, fmt.Sprintf("Cloud Functions API: %s (check project %s)", resp.Status, project))
	}

	return &providers.DoctorResult{
		Provider: p.Name(),
		Checks:   checks,
	}, nil
}
