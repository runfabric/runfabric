package digitalocean

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Invoker invokes the deployed function via HTTP POST to the app URL.
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	url := receipt.Outputs["url"]
	if url == "" {
		url = receipt.Outputs["url_"+function]
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
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke HTTP request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	if resp.StatusCode >= 400 {
		return &providers.InvokeResult{
			Provider: "digitalocean-functions",
			Function: function,
			Output:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out),
		}, nil
	}
	return &providers.InvokeResult{Provider: "digitalocean-functions", Function: function, Output: out}, nil
}
