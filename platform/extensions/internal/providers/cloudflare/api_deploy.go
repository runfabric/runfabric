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

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

const cfAPI = "https://api.cloudflare.com/client/v4"

// Runner deploys a Worker via Cloudflare API (PUT script).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	if apiutil.Env("CLOUDFLARE_ACCOUNT_ID") == "" || apiutil.Env("CLOUDFLARE_API_TOKEN") == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required")
	}
	accountID := apiutil.Env("CLOUDFLARE_ACCOUNT_ID")
	token := apiutil.Env("CLOUDFLARE_API_TOKEN")
	name := fmt.Sprintf("%s-%s", cfg.Service, stage)

	scriptPath := filepath.Join(root, "worker.js")
	if b, _ := os.ReadFile(scriptPath); len(b) == 0 {
		scriptPath = filepath.Join(root, "dist", "worker.js")
	}
	if b, _ := os.ReadFile(scriptPath); len(b) == 0 && len(cfg.Functions) > 0 {
		for _, fn := range cfg.Functions {
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
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cloudflare upload: %s: %s", resp.Status, string(body))
	}
	result := apiutil.BuildDeployResult("cloudflare-workers", cfg, stage)
	resourceID := fmt.Sprintf("accounts/%s/workers/scripts/%s", accountID, name)
	result.Outputs["url"] = fmt.Sprintf("https://%s.workers.dev", name)
	result.Metadata["worker"] = name
	for fnName := range cfg.Functions {
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
