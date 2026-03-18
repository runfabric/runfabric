package gcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Invoke implements providers.Provider by loading the receipt and delegating to Invoker.
// Receipt is loaded from "." (current directory); run from project root so .runfabric/<stage>.json is found.
func (p *Provider) Invoke(cfg *providers.Config, stage, function string, payload []byte) (*providers.InvokeResult, error) {
	receipt, _ := state.Load(".", stage)
	return (Invoker{}).Invoke(context.Background(), cfg, stage, function, payload, receipt)
}

// Invoker invokes via HTTP POST to the function URL from receipt.
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	url := receipt.Outputs["url"]
	if url == "" {
		url = receipt.Outputs["url_"+function]
	}
	if url == "" {
		return nil, fmt.Errorf("no URL in receipt; redeploy first")
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
		return &providers.InvokeResult{Provider: "gcp-functions", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &providers.InvokeResult{Provider: "gcp-functions", Function: function, Output: out}, nil
}
