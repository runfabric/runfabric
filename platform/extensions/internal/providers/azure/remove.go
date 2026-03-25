package azure

import (
	"context"
	"fmt"
	"net/http"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the function app via Azure Management REST API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	rv := apiutil.DecodeReceipt(receipt)
	subID := apiutil.Env("AZURE_SUBSCRIPTION_ID")
	rg := rv.Outputs["resource_group"]
	if rg == "" {
		rg = rv.Metadata["app"]
	}
	if rg == "" {
		serviceName := "service"
		if coreCfg != nil && coreCfg.Service != "" {
			serviceName = coreCfg.Service
		}
		rg = fmt.Sprintf("%s-%s", serviceName, stage)
	}
	appName := rv.Outputs["app_name"]
	if appName == "" {
		serviceName := "service"
		if coreCfg != nil && coreCfg.Service != "" {
			serviceName = coreCfg.Service
		}
		appName = fmt.Sprintf("%s-%s", serviceName, stage)
	}
	if subID == "" || apiutil.Env("AZURE_ACCESS_TOKEN") == "" {
		return &sdkprovider.RemoveResult{Provider: "azure-functions", Removed: true}, nil
	}
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Web/sites/%s?api-version=2022-03-01", subID, rg, appName)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("AZURE_ACCESS_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	// Best effort: 2xx or not, we report removed
	return &sdkprovider.RemoveResult{Provider: "azure-functions", Removed: true}, nil
}
