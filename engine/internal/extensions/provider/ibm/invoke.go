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

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Invoker invokes via OpenWhisk REST API (POST .../actions/...?blocking=true).
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	auth := apiutil.Env("IBM_OPENWHISK_AUTH")
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
	actionName := fmt.Sprintf("%s_%s_%s", cfg.Service, stage, function)
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
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	out := string(b)
	if resp.StatusCode >= 400 {
		return &providers.InvokeResult{Provider: "ibm-openwhisk", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &providers.InvokeResult{Provider: "ibm-openwhisk", Function: function, Output: out}, nil
}
