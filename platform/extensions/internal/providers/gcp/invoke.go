package gcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Invoke loads the receipt and delegates to Invoker.
// Receipt is loaded from "." (current directory); run from project root so .runfabric/<stage>.json is found.
func (p *Provider) Invoke(ctx context.Context, req providers.InvokeRequest) (*providers.InvokeResult, error) {
	receipt, _ := state.Load(".", req.Stage)
	sdkCfg, err := sdkbridge.FromCoreConfig(req.Config)
	if err != nil {
		return nil, err
	}
	r, err := (Invoker{}).Invoke(ctx, sdkCfg, req.Stage, req.Function, req.Payload, receipt)
	if err != nil {
		return nil, err
	}
	return &providers.InvokeResult{
		Provider: r.Provider, Function: r.Function, Output: r.Output,
		RunID: r.RunID, Workflow: r.Workflow,
	}, nil
}

// Invoker invokes via HTTP POST to the function URL from receipt.
type Invoker struct{}

func (Invoker) Invoke(ctx context.Context, cfg sdkprovider.Config, stage, function string, payload []byte, receipt any) (*sdkprovider.InvokeResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	url := rv.Outputs["url"]
	if url == "" {
		url = rv.Outputs["url_"+function]
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
		return &sdkprovider.InvokeResult{Provider: "gcp-functions", Function: function, Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, out)}, nil
	}
	return &sdkprovider.InvokeResult{Provider: "gcp-functions", Function: function, Output: out}, nil
}
