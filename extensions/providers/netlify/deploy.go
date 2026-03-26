// Package netlify implements Netlify API deploy/remove/invoke/logs.
package netlify

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const netlifyAPI = "https://api.netlify.com/api/v1"

// Runner deploys by creating site (if needed) and uploading zip (POST sites, POST deploys).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	service := sdkprovider.Service(cfg)
	if service == "" {
		return nil, fmt.Errorf("service is required in config")
	}
	if sdkprovider.Env("NETLIFY_AUTH_TOKEN") == "" {
		return nil, fmt.Errorf("NETLIFY_AUTH_TOKEN is required")
	}
	siteID := sdkprovider.Env("NETLIFY_SITE_ID")
	if siteID == "" {
		var createResp struct {
			ID string `json:"id"`
		}
		if err := sdkprovider.APIPost(ctx, netlifyAPI+"/sites", "NETLIFY_AUTH_TOKEN", map[string]string{"name": service + "-" + stage}, &createResp); err != nil {
			return nil, fmt.Errorf("create netlify site: %w", err)
		}
		siteID = createResp.ID
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.Contains(rel, "node_modules") || strings.Contains(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		f, _ := w.Create(rel)
		body, _ := os.ReadFile(path)
		_, _ = f.Write(body)
		return nil
	})
	_ = w.Close()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "deploy.zip")
	_, _ = io.Copy(part, &buf)
	_ = mw.Close()
	url := netlifyAPI + "/sites/" + siteID + "/deploys"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("NETLIFY_AUTH_TOKEN"))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("netlify deploy: %s: %s", resp.Status, string(b))
	}
	var out struct {
		ID        string `json:"id"`
		DeployURL string `json:"deploy_ssl_url"`
	}
	_ = json.Unmarshal(b, &out)
	result := sdkprovider.BuildDeployResult("netlify", cfg, stage)
	result.Outputs["url"] = out.DeployURL
	result.Outputs["deploy_id"] = out.ID
	result.Outputs["site_id"] = siteID
	result.Metadata["site_id"] = siteID
	return result, nil
}
