package cloudflare

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Invoker invokes the Worker via HTTP POST to the deployed URL.
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	rv := apiutil.DecodeReceipt(receipt)
	url := rv.Outputs["url"]
	if url == "" {
		url = rv.Outputs["url_"+function]
	}
	if url == "" {
		return nil, fmt.Errorf("no URL in receipt for function %q; redeploy first", function)
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	if resp.StatusCode >= 400 {
		return &sdkprovider.InvokeResult{Provider: "cloudflare-workers", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &sdkprovider.InvokeResult{Provider: "cloudflare-workers", Function: function, Output: out}, nil
}
