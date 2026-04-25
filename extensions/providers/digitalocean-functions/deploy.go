// Package digitalocean implements DigitalOcean App Platform deploy/remove/invoke/logs via API.
package digitalocean

import (
	"context"
	"fmt"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const doAPI = "https://api.digitalocean.com/v2/apps"

// Runner deploys to DigitalOcean App Platform (POST /v2/apps).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	service := sdkprovider.Service(cfg)
	if service == "" {
		return nil, fmt.Errorf("service is required in config")
	}
	if sdkprovider.Env("DIGITALOCEAN_ACCESS_TOKEN") == "" {
		return nil, fmt.Errorf("DIGITALOCEAN_ACCESS_TOKEN is required")
	}
	repo := sdkprovider.Env("DO_APP_REPO")
	if repo == "" {
		return nil, fmt.Errorf("DO_APP_REPO is required (e.g. owner/repo for GitHub)")
	}
	region := sdkprovider.Env("DO_REGION")
	if region == "" {
		region = "ams"
	}
	appName := service + "-" + stage
	runtime := sdkprovider.ProviderRuntime(cfg)
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

	var jobs []map[string]any
	for functionName, expression := range cronSchedules(cfg) {
		expr := strings.TrimSpace(expression)
		if expr == "" {
			expr = "0 * * * *"
		}
		jobs = append(jobs, map[string]any{
			"name":             appName + "-cron-" + functionName,
			"kind":             "SCHEDULED",
			"source_dir":       "/",
			"run_command":      runCommand(runtime),
			"environment_slug": envSlug,
			"schedule":         map[string]any{"cron": expr},
			"github": map[string]any{
				"repo":           repo,
				"branch":         "main",
				"deploy_on_push": true,
			},
		})
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
	if err := sdkprovider.APIPost(ctx, doAPI, "DIGITALOCEAN_ACCESS_TOKEN", map[string]any{"spec": spec}, &out); err != nil {
		return nil, fmt.Errorf("digitalocean apps: %w", err)
	}
	if out.App.ID != "" {
		if err := waitUntilAppActive(ctx, out.App.ID); err != nil {
			return nil, fmt.Errorf("wait for app %s: %w", appName, err)
		}
	}
	result := sdkprovider.BuildDeployResult("digitalocean-functions", cfg, stage)
	result.Outputs["app_id"] = out.App.ID
	result.Outputs["url"] = out.App.LiveURL
	result.Metadata["app_id"] = out.App.ID
	if len(jobs) > 0 {
		result.Metadata["cron_jobs"] = fmt.Sprintf("%d", len(jobs))
	}
	return result, nil
}

func cronSchedules(cfg sdkprovider.Config) map[string]string {
	out := map[string]string{}
	for _, raw := range asSlice(cfg["triggers"]) {
		trigger := asMap(raw)
		if triggerType(trigger) != "cron" {
			continue
		}
		name := "default"
		if s := strings.TrimSpace(sdkprovider.Service(cfg)); s != "" {
			name = s
		}
		out[name] = triggerSchedule(trigger)
	}
	for functionName, fn := range asMap(cfg["functions"]) {
		for _, raw := range asSlice(asMap(fn)["triggers"]) {
			trigger := asMap(raw)
			if triggerType(trigger) != "cron" {
				continue
			}
			out[functionName] = triggerSchedule(trigger)
		}
	}
	return out
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	if s, ok := v.([]map[string]any); ok {
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	}
	return nil
}

func asMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func triggerType(trigger map[string]any) string {
	return strings.ToLower(strings.TrimSpace(fmt.Sprint(trigger["type"])))
}

func triggerSchedule(trigger map[string]any) string {
	for _, key := range []string{"schedule", "expression", "cron"} {
		if value := strings.TrimSpace(fmt.Sprint(trigger[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
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
