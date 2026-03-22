// Package fly implements Fly.io Machines API deploy/remove/invoke/logs.
package fly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

const flyAPI = "https://api.machines.dev/v1"

// Runner deploys by creating the app via Fly Machines API (POST /v1/apps).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	if apiutil.Env("FLY_API_TOKEN") == "" {
		return nil, fmt.Errorf("FLY_API_TOKEN is required")
	}
	org := apiutil.Env("FLY_ORG_ID")
	if org == "" {
		org = "personal"
	}
	appName := fmt.Sprintf("%s-%s", cfg.Service, stage)
	url := flyAPI + "/apps"
	body := struct {
		AppName string `json:"app_name"`
		OrgSlug string `json:"org_slug"`
	}{AppName: appName, OrgSlug: org}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("FLY_API_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fly create app: %s: %s", resp.Status, string(b))
	}
	result := apiutil.BuildDeployResult("fly-machines", cfg, stage)
	result.Outputs["url"] = fmt.Sprintf("https://%s.fly.dev", appName)
	result.Metadata["app"] = appName
	return result, nil
}
