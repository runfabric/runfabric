package ibm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Invoker invokes via OpenWhisk REST API (POST .../actions/...?blocking=true).
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	auth := sdkprovider.Env("IBM_OPENWHISK_AUTH")
	apihost := sdkprovider.Env("IBM_OPENWHISK_API_HOST")
	if apihost == "" {
		apihost = "https://us-south.functions.cloud.ibm.com"
	}
	if !strings.HasPrefix(apihost, "http") {
		apihost = "https://" + apihost
	}
	namespace := sdkprovider.Env("IBM_OPENWHISK_NAMESPACE")
	if namespace == "" {
		namespace = "_"
	}
	actionName := fmt.Sprintf("%s_%s_%s", sdkprovider.Service(cfg), stage, function)
	url := strings.TrimSuffix(apihost, "/") + "/api/v1/namespaces/" + namespace + "/actions/" + actionName + "?blocking=true"
	var input map[string]any
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &input)
	}
	if input == nil {
		input = make(map[string]any)
	}
	body, _ := json.Marshal(map[string]any{"input": input})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	out := string(b)
	if resp.StatusCode >= 400 {
		return &sdkprovider.InvokeResult{Provider: "ibm-openwhisk", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &sdkprovider.InvokeResult{Provider: "ibm-openwhisk", Function: function, Output: out}, nil
}
