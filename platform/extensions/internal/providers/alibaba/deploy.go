package alibaba

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Runner deploys to Alibaba FC via signed OpenAPI (CreateService, CreateFunction, CreateTrigger).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	accessKey := apiutil.Env("ALIBABA_ACCESS_KEY_ID")
	secretKey := apiutil.Env("ALIBABA_ACCESS_KEY_SECRET")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("ALIBABA_ACCESS_KEY_ID and ALIBABA_ACCESS_KEY_SECRET are required")
	}
	accountID := apiutil.Env("ALIBABA_FC_ACCOUNT_ID")
	if accountID == "" {
		return nil, fmt.Errorf("ALIBABA_FC_ACCOUNT_ID is required (Alibaba Cloud account ID)")
	}
	region := coreCfg.Provider.Region
	if region == "" {
		region = apiutil.Env("ALIBABA_FC_REGION")
	}
	if region == "" {
		region = "cn-hangzhou"
	}
	client := newFCClient(accountID, region, accessKey, secretKey)
	serviceName := coreCfg.Service + "-" + stage
	// Ensure service exists (ignore if already exists)
	if _, err := client.CreateService(ctx, serviceName); err != nil &&
		!strings.Contains(err.Error(), "ServiceAlreadyExists") &&
		!strings.Contains(err.Error(), "already exist") &&
		!strings.Contains(err.Error(), "Conflict") {
		return nil, fmt.Errorf("CreateService: %w", err)
	}
	// Deploy each function
	result := apiutil.BuildSDKDeployResult("alibaba-fc", cfg, stage)
	result.Outputs["region"] = region
	result.Outputs["service_name"] = serviceName
	result.Metadata["account_id"] = accountID
	runtime := coreCfg.Provider.Runtime
	if runtime == "" {
		runtime = "nodejs20"
	}
	for fnName, fn := range coreCfg.Functions {
		handler := fn.Handler
		if handler == "" {
			handler = "index.handler"
		}
		memory := 512
		if fn.Memory > 0 {
			memory = fn.Memory
		}
		timeout := 60
		if fn.Timeout > 0 {
			timeout = fn.Timeout
		}
		funcName := fmt.Sprintf("%s-%s-%s", coreCfg.Service, stage, fnName)
		zipBase64, err := zipRoot(root)
		if err != nil {
			return nil, fmt.Errorf("zip root for %s: %w", fnName, err)
		}
		_, err = client.CreateFunction(ctx, serviceName, funcName, handler, runtime, memory, timeout, zipBase64)
		if err != nil && !strings.Contains(err.Error(), "FunctionAlreadyExists") {
			return nil, fmt.Errorf("CreateFunction %s: %w", fnName, err)
		}
		// HTTP trigger so function is invokable via URL
		triggerName := "http"
		_, _ = client.CreateTrigger(ctx, serviceName, funcName, triggerName, "http", map[string]any{
			"authType": "anonymous",
			"methods":  []string{"GET", "POST", "PUT", "DELETE"},
		})
		url := fmt.Sprintf("%s/%s/proxy/%s/%s/", client.baseURL(), fcAPIVersion, serviceName, funcName)
		result.Outputs["url_"+fnName] = url
		if fnName == "" || len(coreCfg.Functions) == 1 {
			result.Outputs["url"] = url
		}
		result.Metadata["function_"+fnName] = funcName
	}
	return result, nil
}

func zipRoot(root string) (string, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
	if err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
