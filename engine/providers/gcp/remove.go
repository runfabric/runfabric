package gcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Remover deletes Cloud Functions via DELETE projects/.../locations/.../functions/...
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	region := receipt.Outputs["region"]
	if region == "" {
		region = "us-central1"
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
