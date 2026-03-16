package api

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
	"github.com/runfabric/runfabric/engine/providers/alibaba"
	"github.com/runfabric/runfabric/engine/providers/azure"
	cf "github.com/runfabric/runfabric/engine/providers/cloudflare"
	do "github.com/runfabric/runfabric/engine/providers/digitalocean"
	"github.com/runfabric/runfabric/engine/providers/fly"
	"github.com/runfabric/runfabric/engine/providers/gcp"
	"github.com/runfabric/runfabric/engine/providers/ibm"
	k8s "github.com/runfabric/runfabric/engine/providers/kubernetes"
	"github.com/runfabric/runfabric/engine/providers/netlify"
	"github.com/runfabric/runfabric/engine/providers/vercel"
)

// Remover deletes deployed resources via provider API.
type Remover interface {
	Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error)
}

var removers = map[string]Remover{
	"digitalocean-functions": &do.Remover{},
	"cloudflare-workers":     &cf.Remover{},
	"vercel":                 &vercel.Remover{},
	"netlify":                &netlify.Remover{},
	"fly-machines":           &fly.Remover{},
	"gcp-functions":          &gcp.Remover{},
	"azure-functions":        &azure.Remover{},
	"kubernetes":             &k8s.Remover{},
	"alibaba-fc":             &alibaba.Remover{},
	"ibm-openwhisk":          &ibm.Remover{},
}

// Remove removes the deployment via provider API and deletes the local receipt.
func Remove(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.RemoveResult, error) {
	remover, ok := removers[provider]
	if !ok {
		return nil, fmt.Errorf("remove via API not implemented for provider %q", provider)
	}
	receipt, err := state.Load(root, stage)
	if err != nil {
		return &providers.RemoveResult{Provider: provider, Removed: true}, nil
	}
	if receipt.Provider != provider {
		return nil, fmt.Errorf("receipt provider %q does not match config provider %q", receipt.Provider, provider)
	}
	result, err := remover.Remove(ctx, cfg, stage, root, receipt)
	if err != nil {
		return nil, err
	}
	_ = state.Delete(root, stage)
	return result, nil
}

// HasRemover returns whether the provider has an API-based remover.
func HasRemover(provider string) bool {
	_, ok := removers[provider]
	return ok
}
