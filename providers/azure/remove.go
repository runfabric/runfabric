package azure

import (
	"context"
	"fmt"
	"net/http"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Remover deletes the function app via Azure Management REST API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	subID := apiutil.Env("AZURE_SUBSCRIPTION_ID")
	rg := receipt.Outputs["resource_group"]
	if rg == "" {
		rg = receipt.Metadata["app"]
	}
	if rg == "" {
		rg = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	appName := receipt.Outputs["app_name"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	if subID == "" || apiutil.Env("AZURE_ACCESS_TOKEN") == "" {
		return &providers.RemoveResult{Provider: "azure-functions", Removed: true}, nil
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
	return &providers.RemoveResult{Provider: "azure-functions", Removed: true}, nil
}
