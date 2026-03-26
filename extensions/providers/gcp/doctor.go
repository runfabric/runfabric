package gcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const listLocationsURL = "https://cloudfunctions.googleapis.com/v2/projects/%s/locations"

// Checker validates credentials and basic API connectivity.
type Checker struct{}

func (Checker) Doctor(ctx context.Context, cfg sdkprovider.Config, stage string) (*sdkprovider.DoctorResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = stage

	project := sdkprovider.Env("GCP_PROJECT")
	if project == "" {
		project = sdkprovider.Env("GCP_PROJECT_ID")
	}
	if project == "" {
		return &sdkprovider.DoctorResult{
			Provider: ProviderID,
			Checks: []string{
				"GCP_PROJECT or GCP_PROJECT_ID is not set (required for deploy/invoke/logs)",
				"See docs/CREDENTIALS.md for GCP setup",
			},
		}, nil
	}

	token := sdkprovider.Env("GCP_ACCESS_TOKEN")
	if token == "" {
		return &sdkprovider.DoctorResult{
			Provider: ProviderID,
			Checks: []string{
				"GCP_ACCESS_TOKEN is not set (e.g. gcloud auth print-access-token or service account key)",
				"Project: " + project,
				"See docs/CREDENTIALS.md for GCP setup",
			},
		}, nil
	}

	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = "us-central1"
	}

	checks := []string{
		"GCP project: " + project,
		"GCP_ACCESS_TOKEN set",
		"Region (config or default): " + region,
		"Runtime: " + sdkprovider.ProviderRuntime(cfg),
	}

	url := fmt.Sprintf(listLocationsURL, project)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(httpReq)
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

	return &sdkprovider.DoctorResult{
		Provider: ProviderID,
		Checks:   checks,
	}, nil
}
