package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Invoker invokes via HTTP POST to the service URL from receipt (e.g. after port-forward or ingress).
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	rv := sdkprovider.DecodeReceipt(receipt)
	url := rv.Outputs["url"]
	if url == "" {
		url = rv.Outputs["url_"+function]
	}
	if url == "" {
		return nil, fmt.Errorf("no URL in receipt; redeploy and ensure url in outputs (e.g. via port-forward or ingress)")
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	if resp.StatusCode >= 400 {
		return &sdkprovider.InvokeResult{Provider: "kubernetes", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &sdkprovider.InvokeResult{Provider: "kubernetes", Function: function, Output: out}, nil
}
