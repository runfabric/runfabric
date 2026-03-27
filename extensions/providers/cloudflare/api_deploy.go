// Package cloudflare implements Cloudflare Workers deploy/remove/invoke/logs via API.
package cloudflare

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

var cfAPI = "https://api.cloudflare.com/client/v4"

// Runner deploys a Worker via Cloudflare API (PUT script).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	service := sdkprovider.Service(cfg)
	if service == "" {
		return nil, fmt.Errorf("service is required in config")
	}
	functions := sdkprovider.Functions(cfg)
	if sdkprovider.Env("CLOUDFLARE_ACCOUNT_ID") == "" || sdkprovider.Env("CLOUDFLARE_API_TOKEN") == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required")
	}
	accountID := sdkprovider.Env("CLOUDFLARE_ACCOUNT_ID")
	token := sdkprovider.Env("CLOUDFLARE_API_TOKEN")
	name := fmt.Sprintf("%s-%s", service, stage)

	scriptPath := filepath.Join(root, "worker.js")
	if b, _ := os.ReadFile(scriptPath); len(b) == 0 {
		scriptPath = filepath.Join(root, "dist", "worker.js")
	}
	if b, _ := os.ReadFile(scriptPath); len(b) == 0 && len(functions) > 0 {
		for _, fn := range functions {
			main := strings.Replace(fn.Handler, ".", "/", 1) + ".js"
			if !strings.HasSuffix(main, ".js") {
				main += ".js"
			}
			scriptPath = filepath.Join(root, main)
			break
		}
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("read worker script from %s: %w (build your worker first)", scriptPath, err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("worker.js", "worker.js")
	_, _ = part.Write(script)
	_ = w.WriteField("main", "worker.js")
	_ = w.Close()

	url := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + name
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cloudflare upload: %s: %s", resp.Status, string(body))
	}
	result := sdkprovider.BuildDeployResult("cloudflare-workers", cfg, stage)
	resourceID := fmt.Sprintf("accounts/%s/workers/scripts/%s", accountID, name)
	result.Outputs["url"] = fmt.Sprintf("https://%s.workers.dev", name)
	result.Metadata["worker"] = name
	for fnName := range functions {
		deployed := result.Functions[fnName]
		deployed.ResourceName = name
		deployed.ResourceIdentifier = resourceID
		deployed.Metadata = map[string]string{
			"accountId": accountID,
			"worker":    name,
		}
		result.Functions[fnName] = deployed
	}
	return result, nil
}
