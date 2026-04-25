// Package vercel implements Vercel API deploy/remove/invoke/logs.
package vercel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const vercelAPI = "https://api.vercel.com"

// Runner deploys via Vercel API (POST /v13/deployments with files).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	projectName := sdkprovider.Service(cfg)
	if projectName == "" {
		return nil, fmt.Errorf("service is required in config")
	}
	if sdkprovider.Env("VERCEL_TOKEN") == "" {
		return nil, fmt.Errorf("VERCEL_TOKEN is required")
	}
	teamID := sdkprovider.Env("VERCEL_TEAM_ID")
	type vercelFile struct {
		File     string `json:"file"`
		Content  string `json:"content,omitempty"`
		Encoding string `json:"encoding,omitempty"`
	}
	var files []vercelFile
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.Contains(rel, "node_modules") || strings.Contains(rel, ".git") {
			return filepath.SkipDir
		}
		if len(files) >= 100 {
			return filepath.SkipAll
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		files = append(files, vercelFile{File: rel, Content: string(body), Encoding: "utf-8"})
		return nil
	})
	if len(files) == 0 {
		if b, _ := os.ReadFile(filepath.Join(root, "package.json")); len(b) > 0 {
			files = append(files, vercelFile{File: "package.json", Content: string(b), Encoding: "utf-8"})
		}
	}
	payload := map[string]any{"name": projectName, "files": files, "target": stage}
	if teamID != "" {
		payload["teamId"] = teamID
	}
	bodyBytes, _ := json.Marshal(payload)
	url := vercelAPI + "/v13/deployments"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("VERCEL_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vercel deploy: %s: %s", resp.Status, string(b))
	}
	var out struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	_ = json.Unmarshal(b, &out)
	// Vercel deployments are async — poll until readyState == READY.
	if out.ID != "" {
		if err := waitUntilDeploymentReady(ctx, out.ID, teamID); err != nil {
			return nil, fmt.Errorf("wait for deployment %s: %w", out.ID, err)
		}
	}
	result := sdkprovider.BuildDeployResult("vercel", cfg, stage)
	if out.URL != "" {
		if !strings.HasPrefix(out.URL, "http") {
			out.URL = "https://" + out.URL
		}
		result.Outputs["url"] = out.URL
	} else {
		result.Outputs["url"] = "https://" + projectName + ".vercel.app"
	}
	result.Outputs["deployment_id"] = out.ID
	result.Metadata["project"] = projectName
	return result, nil
}
