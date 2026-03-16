package digitalocean

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
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
