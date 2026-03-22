package cloudflare

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Invoker invokes the Worker via HTTP POST to the deployed URL.
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	url := receipt.Outputs["url"]
	if url == "" {
		url = receipt.Outputs["url_"+function]
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
		return &providers.InvokeResult{Provider: "cloudflare-workers", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &providers.InvokeResult{Provider: "cloudflare-workers", Function: function, Output: out}, nil
}
