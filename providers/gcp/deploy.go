// Package gcp implements GCP Cloud Functions v2 API deploy/remove/invoke/logs.
package gcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
)

const gcpAPI = "https://cloudfunctions.googleapis.com/v2"

// Runner deploys via Cloud Functions v2 REST API (requires GCP_SOURCE_BUCKET/OBJECT or upload pipeline).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	if apiutil.Env("GCP_ACCESS_TOKEN") == "" {
		return nil, fmt.Errorf("GCP_ACCESS_TOKEN is required (e.g. from gcloud auth print-access-token or a service account)")
	}
	if apiutil.Env("GCP_PROJECT") == "" {
		return nil, fmt.Errorf("GCP_PROJECT is required")
	}
	region := cfg.Provider.Region
	if region == "" {
		region = "us-central1"
	}
	if apiutil.Env("GCP_SOURCE_BUCKET") == "" {
		return nil, fmt.Errorf("GCP Cloud Functions API requires pre-uploaded source: set GCP_SOURCE_BUCKET and GCP_SOURCE_OBJECT (upload your app first), or use a build pipeline")
	}
	result := apiutil.BuildDeployResult("gcp-functions", cfg, stage)
	result.Outputs["region"] = region
	project := apiutil.Env("GCP_PROJECT")
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
				"runtime":     "nodejs20",
				"entryPoint": entryPoint,
				"source": map[string]any{
					"storageSource": map[string]any{
						"bucket": apiutil.Env("GCP_SOURCE_BUCKET"),
						"object": apiutil.Env("GCP_SOURCE_OBJECT"),
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
