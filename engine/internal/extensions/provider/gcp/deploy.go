// Package gcp implements GCP Cloud Functions v2 API deploy/remove/invoke/logs.
package gcp

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
)

// Deploy implements providers.Provider by delegating to Runner (Cloud Functions v2 API).
func (p *Provider) Deploy(cfg *providers.Config, stage, root string) (*providers.DeployResult, error) {
	return (Runner{}).Deploy(context.Background(), cfg, stage, root)
}

const gcpAPI = "https://cloudfunctions.googleapis.com/v2"
const gcsUploadAPI = "https://storage.googleapis.com/upload/storage/v1/b"

// Runner deploys via Cloud Functions v2 REST API. Source can be pre-uploaded (GCP_SOURCE_BUCKET + GCP_SOURCE_OBJECT)
// or uploaded automatically: set GCP_UPLOAD_BUCKET to zip project root and upload before deploy.
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	if apiutil.Env("GCP_ACCESS_TOKEN") == "" {
		return nil, fmt.Errorf("GCP_ACCESS_TOKEN is required (e.g. from gcloud auth print-access-token or a service account)")
	}
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT or GCP_PROJECT_ID is required")
	}
	region := cfg.Provider.Region
	if region == "" {
		region = "us-central1"
	}

	bucket := apiutil.Env("GCP_SOURCE_BUCKET")
	object := apiutil.Env("GCP_SOURCE_OBJECT")
	if bucket == "" && apiutil.Env("GCP_UPLOAD_BUCKET") != "" {
		uploadBucket := apiutil.Env("GCP_UPLOAD_BUCKET")
		objName := fmt.Sprintf("runfabric-%s-%s-%d.zip", cfg.Service, stage, time.Now().Unix())
		if err := uploadZipToGCS(ctx, root, uploadBucket, objName); err != nil {
			return nil, fmt.Errorf("upload source to GCS: %w", err)
		}
		bucket = uploadBucket
		object = objName
	}
	if bucket == "" || object == "" {
		return nil, fmt.Errorf("GCP Cloud Functions requires source in GCS: set GCP_SOURCE_BUCKET and GCP_SOURCE_OBJECT, or set GCP_UPLOAD_BUCKET to auto-upload from project root")
	}

	result := apiutil.BuildDeployResult("gcp-functions", cfg, stage)
	result.Outputs["region"] = region
	for fnName, fn := range cfg.Functions {
		entryPoint := "handler"
		if fn.Handler != "" {
			entryPoint = strings.Split(fn.Handler, ".")[0]
		}
		funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fnName)
		parent := fmt.Sprintf("projects/%s/locations/%s", project, region)
		url := gcpAPI + "/" + parent + "/functions?functionId=" + funcName
		body := map[string]any{
			"name":        parent + "/functions/" + funcName,
			"environment": "GEN_2",
			"buildConfig": map[string]any{
				"runtime":    "nodejs20",
				"entryPoint": entryPoint,
				"source": map[string]any{
					"storageSource": map[string]any{
						"bucket": bucket,
						"object": object,
					},
				},
			},
			"serviceConfig": map[string]any{"availableMemory": "256Mi"},
		}
		bodyBytes, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+apiutil.Env("GCP_ACCESS_TOKEN"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := apiutil.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return nil, fmt.Errorf("gcp functions deploy %s: %s: %s", fnName, resp.Status, string(b))
		}
		var fnResp struct {
			ServiceConfig struct {
				URI string `json:"uri"`
			} `json:"serviceConfig"`
		}
		_ = json.Unmarshal(b, &fnResp)
		if fnResp.ServiceConfig.URI != "" {
			result.Outputs["url_"+fnName] = fnResp.ServiceConfig.URI
		}
	}
	return result, nil
}

// uploadZipToGCS zips root (excluding node_modules, .git) and uploads to GCS via REST.
func uploadZipToGCS(ctx context.Context, root, bucket, objectName string) error {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(path, "node_modules"+string(filepath.Separator)) || strings.Contains(path, ".git"+string(filepath.Separator)) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		f, err := w.Create(rel)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = f.Write(body)
		return err
	})
	if err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	uploadURL := fmt.Sprintf("%s/%s/o?uploadType=media&name=%s", gcsUploadAPI, bucket, url.QueryEscape(objectName))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("GCP_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/zip")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GCS upload: %s: %s", resp.Status, string(body))
	}
	return nil
}
