package digitalocean

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Invoker invokes the deployed function via HTTP POST to the app URL.
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	rv := sdkprovider.DecodeReceipt(receipt)
	url := rv.Outputs["url"]
	if url == "" {
		url = rv.Outputs["url_"+function]
	}
	if url == "" {
		return nil, fmt.Errorf("no URL in receipt for function %q; redeploy and ensure provider writes url to outputs", function)
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke HTTP request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	if resp.StatusCode >= 400 {
		return &sdkprovider.InvokeResult{
			Provider: "digitalocean-functions",
			Function: function,
			Output:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out),
		}, nil
	}
	return &sdkprovider.InvokeResult{Provider: "digitalocean-functions", Function: function, Output: out}, nil
}
