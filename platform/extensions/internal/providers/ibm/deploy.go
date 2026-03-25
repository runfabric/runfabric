// Package ibm implements IBM OpenWhisk API deploy/remove/invoke/logs.
package ibm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Runner deploys actions via OpenWhisk REST API (PUT /api/v1/namespaces/.../actions/...).
type Runner struct{}

func (Runner) Deploy(ctx context.Context, cfg sdkprovider.Config, stage, root string) (*sdkprovider.DeployResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	if apiutil.Env("IBM_OPENWHISK_AUTH") == "" {
		return nil, fmt.Errorf("IBM_OPENWHISK_AUTH is required (e.g. user:password or API key)")
	}
	apihost := apiutil.Env("IBM_OPENWHISK_API_HOST")
	if apihost == "" {
		apihost = "https://us-south.functions.cloud.ibm.com"
	}
	if !strings.HasPrefix(apihost, "http") {
		apihost = "https://" + apihost
	}
	namespace := apiutil.Env("IBM_OPENWHISK_NAMESPACE")
	if namespace == "" {
		namespace = "_"
	}
	auth := apiutil.Env("IBM_OPENWHISK_AUTH")
	result := apiutil.BuildSDKDeployResult("ibm-openwhisk", cfg, stage)
	result.Metadata["namespace"] = namespace
	baseURL := strings.TrimSuffix(apihost, "/") + "/api/v1/namespaces/" + namespace + "/actions/"
	for fnName, fn := range coreCfg.Functions {
		handlerPath := fn.Handler
		if handlerPath == "" {
			handlerPath = "index.handler"
		}
		mainJS := filepath.Join(root, "index.js")
		if idx := strings.Index(handlerPath, "."); idx > 0 {
			mainJS = filepath.Join(root, strings.ReplaceAll(handlerPath[:idx], ".", string(filepath.Separator))+".js")
		}
		code, err := os.ReadFile(mainJS)
		if err != nil {
			return nil, fmt.Errorf("read action code %s: %w", mainJS, err)
		}
		actionName := fmt.Sprintf("%s_%s_%s", coreCfg.Service, stage, fnName)
		url := baseURL + actionName + "?overwrite=true"
		body := map[string]any{"exec": map[string]any{"kind": "nodejs:20", "code": string(code)}}
		bodyBytes, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Basic "+base64Encode(auth))
		resp, err := apiutil.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("openwhisk create action %s: %s: %s", fnName, resp.Status, string(b))
		}
		result.Outputs["action_"+fnName] = apihost + "/api/v1/namespaces/" + namespace + "/actions/" + actionName
	}
	return result, nil
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
