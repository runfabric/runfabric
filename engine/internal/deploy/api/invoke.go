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

// Invoker invokes a deployed function via provider API or HTTP.
type Invoker interface {
	Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error)
}

var invokers = map[string]Invoker{
	"digitalocean-functions": &do.Invoker{},
	"cloudflare-workers":     &cf.Invoker{},
	"vercel":                 &vercel.Invoker{},
	"netlify":                &netlify.Invoker{},
	"fly-machines":           &fly.Invoker{},
	"gcp-functions":          &gcp.Invoker{},
	"azure-functions":        &azure.Invoker{},
	"kubernetes":             &k8s.Invoker{},
	"alibaba-fc":             &alibaba.Invoker{},
	"ibm-openwhisk":          &ibm.Invoker{},
}

// Invoke invokes the deployed function via provider API.
func Invoke(ctx context.Context, provider string, cfg *config.Config, stage, function string, payload []byte, root string) (*providers.InvokeResult, error) {
	invoker, ok := invokers[provider]
	if !ok {
		return nil, fmt.Errorf("invoke via API not implemented for provider %q", provider)
	}
	receipt, err := state.Load(root, stage)
	if err != nil {
		return nil, fmt.Errorf("no deployment found for stage %q (run deploy first): %w", stage, err)
	}
	if receipt.Provider != provider {
		return nil, fmt.Errorf("receipt provider %q does not match %q", receipt.Provider, provider)
	}
	return invoker.Invoke(ctx, cfg, stage, function, payload, receipt)
}

// HasInvoker returns whether the provider has an API-based invoker.
func HasInvoker(provider string) bool {
	_, ok := invokers[provider]
	return ok
}
