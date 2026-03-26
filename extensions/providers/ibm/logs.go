package ibm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Logger fetches activations and logs via OpenWhisk API (GET .../activations?name=...).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
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
	url := strings.TrimSuffix(apihost, "/") + "/api/v1/namespaces/" + namespace + "/activations?limit=20&name=" + actionName
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return &sdkprovider.LogsResult{Provider: "ibm-openwhisk", Function: function, Lines: []string{err.Error()}}, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return &sdkprovider.LogsResult{Provider: "ibm-openwhisk", Function: function, Lines: []string{string(b)}}, nil
	}
	var out struct {
		Activations []struct {
			ActivationID string   `json:"activationId"`
			Logs         []string `json:"logs"`
		} `json:"activations"`
	}
	if json.Unmarshal(b, &out) != nil {
		return &sdkprovider.LogsResult{Provider: "ibm-openwhisk", Function: function, Lines: []string{string(b)}}, nil
	}
	var lines []string
	for _, a := range out.Activations {
		lines = append(lines, "--- "+a.ActivationID+" ---")
		lines = append(lines, a.Logs...)
	}
	if len(lines) == 0 {
		lines = []string{"No recent activations."}
	}
	return &sdkprovider.LogsResult{Provider: "ibm-openwhisk", Function: function, Lines: lines}, nil
}
