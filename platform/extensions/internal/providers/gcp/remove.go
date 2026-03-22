package gcp

import (
	"context"
	"fmt"
	"net/http"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Remove loads the receipt and delegates to Remover.
func (p *Provider) Remove(ctx context.Context, req providers.RemoveRequest) (*providers.RemoveResult, error) {
	receipt, _ := state.Load(req.Root, req.Stage)
	return (Remover{}).Remove(ctx, req.Config, req.Stage, req.Root, receipt)
}

// Remover deletes Cloud Functions via DELETE projects/.../locations/.../functions/...
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	region := "us-central1"
	if receipt != nil && receipt.Outputs != nil && receipt.Outputs["region"] != "" {
		region = receipt.Outputs["region"]
	}
	if project == "" || apiutil.Env("GCP_ACCESS_TOKEN") == "" {
		return &providers.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
	}
	token := apiutil.Env("GCP_ACCESS_TOKEN")
	for fnName := range cfg.Functions {
		funcName := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fnName)
		url := fmt.Sprintf("%s/projects/%s/locations/%s/functions/%s", gcpAPI, project, region, funcName)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		apiutil.DefaultClient.Do(req) // best effort
	}
	return &providers.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
}
