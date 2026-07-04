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

	"github.com/runfabric/runfabric/plugin-sdk/go/gcpauth"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const gcpAPI = "https://cloudfunctions.googleapis.com/v2"
const gcsUploadAPI = "https://storage.googleapis.com/upload/storage/v1/b"

// Runner deploys via Cloud Functions v2 REST API. Source can be pre-uploaded (GCP_SOURCE_BUCKET + GCP_SOURCE_OBJECT)
// or uploaded automatically: set GCP_UPLOAD_BUCKET to zip project root and upload before deploy.
type Runner struct{}

func (r Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	service := strings.TrimSpace(sdkprovider.Service(cfg))
	if service == "" {
		return nil, fmt.Errorf("invalid config")
	}
	// Resolves GCP_ACCESS_TOKEN from the env or a GOOGLE_APPLICATION_CREDENTIALS
	// service-account key (minted + auto-refreshed by the plugin SDK).
	if err := gcpauth.EnsureAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("gcp credentials: %w", err)
	}
	project := sdkprovider.Env("GCP_PROJECT")
	if project == "" {
		project = sdkprovider.Env("GCP_PROJECT_ID")
	}
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT or GCP_PROJECT_ID is required")
	}
	region := strings.TrimSpace(sdkprovider.ProviderRegion(cfg))
	if region == "" {
		region = "us-central1"
	}

	bucket := sdkprovider.Env("GCP_SOURCE_BUCKET")
	object := sdkprovider.Env("GCP_SOURCE_OBJECT")
	if bucket == "" && sdkprovider.Env("GCP_UPLOAD_BUCKET") != "" {
		uploadBucket := sdkprovider.Env("GCP_UPLOAD_BUCKET")
		objName := fmt.Sprintf("runfabric-%s-%s-%d.zip", service, stage, time.Now().Unix())
		if err := uploadZipToGCS(ctx, root, uploadBucket, objName); err != nil {
			return nil, fmt.Errorf("upload source to GCS: %w", err)
		}
		bucket = uploadBucket
		object = objName
	}
	if bucket == "" || object == "" {
		return nil, fmt.Errorf("GCP Cloud Functions requires source in GCS: set GCP_SOURCE_BUCKET and GCP_SOURCE_OBJECT, or set GCP_UPLOAD_BUCKET to auto-upload from project root")
	}

	defaultRuntime := normalizeGcpRuntime(sdkprovider.ProviderRuntime(cfg))

	result := sdkprovider.BuildDeployResult(ProviderID, cfg, stage)
	result.Outputs["region"] = region
	for fnName, fn := range sdkprovider.Functions(cfg) {
		entryPoint := "handler"
		if fn.Handler != "" {
			entryPoint = strings.Split(fn.Handler, ".")[0]
		}
		runtime := defaultRuntime
		if fn.Runtime != "" {
			runtime = normalizeGcpRuntime(fn.Runtime)
		}
		funcName := fmt.Sprintf("%s-%s-%s", service, stage, fnName)
		parent := fmt.Sprintf("projects/%s/locations/%s", project, region)
		resourceName := parent + "/functions/" + funcName
		url := gcpAPI + "/" + parent + "/functions?functionId=" + funcName
		body := map[string]any{
			"name":        resourceName,
			"environment": "GEN_2",
			"buildConfig": map[string]any{
				"runtime":    runtime,
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
		req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("GCP_ACCESS_TOKEN"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := sdkprovider.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return nil, fmt.Errorf("gcp functions deploy %s: %s: %s", fnName, resp.Status, string(b))
		}
		// Cloud Functions v2 deploy returns a Long-Running Operation — poll until done.
		var opResp struct {
			Name          string `json:"name"`
			ServiceConfig struct {
				URI string `json:"uri"`
			} `json:"serviceConfig"`
		}
		_ = json.Unmarshal(b, &opResp)
		if opResp.Name != "" {
			if err := waitUntilFunctionReady(ctx, opResp.Name); err != nil {
				return nil, fmt.Errorf("wait for function %s: %w", fnName, err)
			}
		}
		fnResp := opResp
		deployed := result.Functions[fnName]
		deployed.ResourceName = funcName
		deployed.ResourceIdentifier = resourceName
		if fnResp.ServiceConfig.URI != "" {
			deployed.ResourceIdentifier = fnResp.ServiceConfig.URI
		}
		deployed.Metadata = map[string]string{
			"project":      project,
			"region":       region,
			"resourceName": resourceName,
		}
		result.Functions[fnName] = deployed
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
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("GCP_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/zip")
	resp, err := sdkprovider.DefaultClient.Do(req)
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

// normalizeGcpRuntime maps short/other-cloud runtime names onto Cloud Functions
// gen-2 runtime ids (e.g. nodejs → nodejs20, nodejs20.x → nodejs20,
// python/python3.12 → python312, go → go121). Unknown values pass through so
// explicit GCP runtime ids keep working.
func normalizeGcpRuntime(runtime string) string {
	rt := strings.ToLower(strings.TrimSpace(runtime))
	rt = strings.TrimSuffix(rt, ".x")
	switch {
	case rt == "" || rt == "nodejs" || rt == "node":
		return "nodejs20"
	case rt == "python":
		return "python312"
	case rt == "go" || rt == "golang":
		return "go121"
	case strings.HasPrefix(rt, "python3."):
		// python3.12 → python312
		return "python" + strings.ReplaceAll(strings.TrimPrefix(rt, "python"), ".", "")
	default:
		return rt
	}
}
