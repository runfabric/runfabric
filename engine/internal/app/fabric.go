package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// FabricDeploy runs deploy for each target in cfg.Fabric.Targets (provider keys), collects the primary URL from each receipt, and saves fabric state.
// Requires cfg.Fabric and cfg.ProviderOverrides; each target must be a key in ProviderOverrides.
func FabricDeploy(configPath, stage string, rollbackOnFailure, noRollbackOnFailure bool) (*state.FabricState, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	if ctx.Config.Fabric == nil || len(ctx.Config.Fabric.Targets) == 0 {
		return nil, nil
	}
	if ctx.Config.ProviderOverrides == nil {
		return nil, nil
	}

	var endpoints []state.FabricEndpoint
	for _, providerKey := range ctx.Config.Fabric.Targets {
		if _, ok := ctx.Config.ProviderOverrides[providerKey]; !ok {
			continue
		}
		_, err := Deploy(configPath, stage, "", rollbackOnFailure, noRollbackOnFailure, nil, providerKey)
		if err != nil {
			return nil, err
		}
		receipt, err := ctx.Backends.Receipts.Load(stage)
		if err != nil || receipt == nil {
			continue
		}
		url := ServiceURLFromReceipt(receipt.Outputs)
		if url == "" {
			for _, v := range receipt.Outputs {
				if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
					url = v
					break
				}
			}
		}
		endpoints = append(endpoints, state.FabricEndpoint{
			Provider:  providerKey,
			URL:       url,
			UpdatedAt: receipt.UpdatedAt,
		})
	}

	fabricState := &state.FabricState{
		Service:   ctx.Config.Service,
		Stage:     stage,
		Endpoints: endpoints,
	}
	if err := state.SaveFabricState(ctx.RootDir, fabricState); err != nil {
		return nil, err
	}
	return fabricState, nil
}

// FabricHealth runs HTTP GET on each endpoint in fabric state and sets Healthy. Uses cfg if non-nil for optional health check URL path.
func FabricHealth(configPath, stage string) (*state.FabricState, error) {
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, err
	}
	fabricState, err := state.LoadFabricState(ctx.RootDir, stage)
	if err != nil || fabricState == nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	for i := range fabricState.Endpoints {
		url := fabricState.Endpoints[i].URL
		if url == "" {
			falseVal := false
			fabricState.Endpoints[i].Healthy = &falseVal
			continue
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			falseVal := false
			fabricState.Endpoints[i].Healthy = &falseVal
			continue
		}
		resp, err := client.Do(req)
		ok := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
		if resp != nil {
			resp.Body.Close()
		}
		fabricState.Endpoints[i].Healthy = &ok
	}
	return fabricState, nil
}

// FabricTargets returns the list of provider keys to use for fabric deploy. If cfg has no fabric or no targets, returns nil.
func FabricTargets(cfg *config.Config) []string {
	if cfg == nil || cfg.Fabric == nil || len(cfg.Fabric.Targets) == 0 {
		return nil
	}
	return cfg.Fabric.Targets
}
