// Package digitalocean implements DigitalOcean App Platform deploy/remove/invoke/logs via API.
package digitalocean

import (
	"context"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/planner"
	"github.com/runfabric/runfabric/internal/providers"
)

const doAPI = "https://api.digitalocean.com/v2/apps"

// Runner deploys to DigitalOcean App Platform (POST /v2/apps).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	if apiutil.Env("DIGITALOCEAN_ACCESS_TOKEN") == "" {
		return nil, fmt.Errorf("DIGITALOCEAN_ACCESS_TOKEN is required")
	}
	repo := apiutil.Env("DO_APP_REPO")
	if repo == "" {
		return nil, fmt.Errorf("DO_APP_REPO is required (e.g. owner/repo for GitHub)")
	}
	region := apiutil.Env("DO_REGION")
	if region == "" {
		region = "ams"
	}
	appName := cfg.Service + "-" + stage
	runtime := cfg.Provider.Runtime
	if runtime == "" {
		runtime = "nodejs"
	}
	envSlug := environmentSlug(runtime)

	spec := map[string]any{
		"name":   appName,
		"region": region,
		"functions": []map[string]any{{
			"name":             appName + "-fn",
			"source_dir":       "/",
			"environment_slug": envSlug,
			"github": map[string]any{
				"repo":           repo,
				"branch":         "main",
				"deploy_on_push": true,
			},
		}},
	}

	triggers := planner.ExtractTriggers(cfg)
	var jobs []map[string]any
	for _, ft := range triggers {
		for _, s := range ft.Specs {
			if s.Kind != planner.TriggerCron {
				continue
			}
			expr, _ := s.Config["expression"].(string)
			if expr == "" {
				expr = "0 * * * *"
			}
			jobs = append(jobs, map[string]any{
				"name":             appName + "-cron-" + ft.Function,
				"kind":             "SCHEDULED",
				"source_dir":       "/",
				"run_command":     runCommand(runtime),
				"environment_slug": envSlug,
				"schedule":         map[string]any{"cron": expr},
				"github": map[string]any{
					"repo":           repo,
					"branch":         "main",
					"deploy_on_push": true,
				},
			})
		}
	}
	if len(jobs) > 0 {
		spec["jobs"] = jobs
	}

	var out struct {
		App struct {
			ID      string `json:"id"`
			LiveURL string `json:"live_url"`
		} `json:"app"`
	}
	if err := apiutil.APIPost(ctx, doAPI, "DIGITALOCEAN_ACCESS_TOKEN", map[string]any{"spec": spec}, &out); err != nil {
		return nil, fmt.Errorf("digitalocean apps: %w", err)
	}
	result := apiutil.BuildDeployResult("digitalocean-functions", cfg, stage)
	result.Outputs["app_id"] = out.App.ID
	result.Outputs["url"] = out.App.LiveURL
	result.Metadata["app_id"] = out.App.ID
	if len(jobs) > 0 {
		result.Metadata["cron_jobs"] = fmt.Sprintf("%d", len(jobs))
	}
	return result, nil
}

func environmentSlug(runtime string) string {
	switch strings.ToLower(runtime) {
	case "go", "golang":
		return "go"
	case "python", "py":
		return "python"
	default:
		return "node-js"
	}
}

func runCommand(runtime string) string {
	switch strings.ToLower(runtime) {
	case "go", "golang":
		return "./bin/app"
	case "python", "py":
		return "python index.py"
	default:
		return "node index.js"
	}
}
