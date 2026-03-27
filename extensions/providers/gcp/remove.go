package gcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes Cloud Functions via DELETE projects/.../locations/.../functions/...
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	service := strings.TrimSpace(sdkprovider.Service(cfg))
	functions := sdkprovider.Functions(cfg)
	rv := sdkprovider.DecodeReceipt(receipt)
	project := sdkprovider.Env("GCP_PROJECT")
	if project == "" {
		project = sdkprovider.Env("GCP_PROJECT_ID")
	}
	region := "us-central1"
	if rv.Outputs["region"] != "" {
		region = rv.Outputs["region"]
	}
	if project == "" || sdkprovider.Env("GCP_ACCESS_TOKEN") == "" {
		return &sdkprovider.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
	}
	if service == "" {
		return &sdkprovider.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
	}
	token := sdkprovider.Env("GCP_ACCESS_TOKEN")
	for fnName := range functions {
		funcName := fmt.Sprintf("%s-%s-%s", service, stage, fnName)
		url := fmt.Sprintf("%s/projects/%s/locations/%s/functions/%s", gcpAPI, project, region, funcName)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		sdkprovider.DefaultClient.Do(req) // best effort
	}
	return &sdkprovider.RemoveResult{Provider: "gcp-functions", Removed: true}, nil
}
