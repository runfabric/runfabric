// Package fly implements Fly.io Machines API deploy/remove/invoke/logs.
package fly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const flyAPI = "https://api.machines.dev/v1"

// Runner deploys by creating the Fly app and launching a Machine via the Machines API.
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	service := sdkprovider.Service(cfg)
	if service == "" {
		return nil, fmt.Errorf("service is required in config")
	}
	if sdkprovider.Env("FLY_API_TOKEN") == "" {
		return nil, fmt.Errorf("FLY_API_TOKEN is required")
	}
	image := sdkprovider.Env("FLY_IMAGE")
	if image == "" {
		return nil, fmt.Errorf("FLY_IMAGE is required (e.g. registry.fly.io/myapp:latest)")
	}
	org := sdkprovider.Env("FLY_ORG_ID")
	if org == "" {
		org = "personal"
	}
	appName := fmt.Sprintf("%s-%s", service, stage)

	// Step 1: create the app (idempotent — 409 Conflict means already exists).
	if err := createApp(ctx, appName, org); err != nil {
		return nil, err
	}

	// Step 2: launch a machine with the specified image.
	machineID, err := createMachine(ctx, appName, image)
	if err != nil {
		return nil, err
	}

	// Step 3: wait until the machine is started and ready to serve traffic.
	if err := waitUntilMachineStarted(ctx, appName, machineID); err != nil {
		return nil, fmt.Errorf("wait for machine %s: %w", machineID, err)
	}

	result := sdkprovider.BuildDeployResult("fly-machines", cfg, stage)
	result.Outputs["url"] = fmt.Sprintf("https://%s.fly.dev", appName)
	result.Metadata["app"] = appName
	result.Metadata["machine_id"] = machineID
	return result, nil
}

func createApp(ctx context.Context, appName, org string) error {
	body := struct {
		AppName string `json:"app_name"`
		OrgSlug string `json:"org_slug"`
	}{AppName: appName, OrgSlug: org}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, flyAPI+"/apps", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("FLY_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fly create app: %s: %s", resp.Status, string(b))
	}
	return nil
}

func createMachine(ctx context.Context, appName, image string) (string, error) {
	body := map[string]any{
		"config": map[string]any{
			"image": image,
			"services": []map[string]any{{
				"ports":         []map[string]any{{"port": 443, "handlers": []string{"tls", "http"}}},
				"protocol":      "tcp",
				"internal_port": 8080,
			}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	url := flyAPI + "/apps/" + appName + "/machines"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("FLY_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("fly create machine: %s: %s", resp.Status, string(b))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("fly create machine: could not parse machine ID from response")
	}
	return out.ID, nil
}
