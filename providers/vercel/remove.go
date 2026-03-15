package vercel

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Remover deletes the project via Vercel API (DELETE /v9/projects/{name}).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	projectName := receipt.Metadata["project"]
	if projectName == "" {
		projectName = cfg.Service
	}
	url := vercelAPI + "/v9/projects/" + projectName
	if err := apiutil.DoDelete(ctx, url, "VERCEL_TOKEN"); err != nil {
		return nil, fmt.Errorf("vercel delete project: %w", err)
	}
	return &providers.RemoveResult{Provider: "vercel", Removed: true}, nil
}
