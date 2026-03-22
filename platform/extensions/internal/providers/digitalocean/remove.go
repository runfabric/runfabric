package digitalocean

import (
	"context"
	"fmt"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Remover deletes the app via DigitalOcean API.
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	appID := receipt.Outputs["app_id"]
	if appID == "" {
		return nil, fmt.Errorf("receipt missing app_id; cannot remove DigitalOcean app")
	}
	url := doAPI + "/" + appID
	if err := apiutil.DoDelete(ctx, url, "DIGITALOCEAN_ACCESS_TOKEN"); err != nil {
		return nil, fmt.Errorf("digitalocean delete app: %w", err)
	}
	return &providers.RemoveResult{Provider: "digitalocean-functions", Removed: true}, nil
}
