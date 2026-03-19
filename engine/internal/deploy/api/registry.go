package api

import (
	"context"
	"fmt"
	"sort"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/alibaba"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/azure"
	cf "github.com/runfabric/runfabric/engine/internal/extensions/provider/cloudflare"
	do "github.com/runfabric/runfabric/engine/internal/extensions/provider/digitalocean"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/fly"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/gcp"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/ibm"
	k8s "github.com/runfabric/runfabric/engine/internal/extensions/provider/kubernetes"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/netlify"
	"github.com/runfabric/runfabric/engine/internal/extensions/provider/vercel"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Provider is the unified internal API-dispatch interface used by deploy/api.
// Non-AWS providers should be standardized behind this interface.
type Provider interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
	Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error)
	Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error)
	Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error)
}

// Legacy capability interfaces are preserved as migration shims while provider adapters
// are being standardized behind the unified Provider interface.
type Runner interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
}

type Remover interface {
	Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error)
}

type Invoker interface {
	Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error)
}

type Logger interface {
	Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error)
}

type legacyProviderShim struct {
	name    string
	runner  Runner
	remover Remover
	invoker Invoker
	logger  Logger
}

func (p *legacyProviderShim) Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	return p.runner.Deploy(ctx, cfg, stage, root)
}

func (p *legacyProviderShim) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	return p.remover.Remove(ctx, cfg, stage, root, receipt)
}

func (p *legacyProviderShim) Invoke(ctx context.Context, cfg *config.Config, stage, function string, payload []byte, receipt *state.Receipt) (*providers.InvokeResult, error) {
	return p.invoker.Invoke(ctx, cfg, stage, function, payload, receipt)
}

func (p *legacyProviderShim) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	return p.logger.Logs(ctx, cfg, stage, function, receipt)
}

func mustLegacyProvider(name string, runner Runner, remover Remover, invoker Invoker, logger Logger) Provider {
	if runner == nil || remover == nil || invoker == nil || logger == nil {
		panic(fmt.Sprintf("api provider %q missing required capability (deploy/remove/invoke/logs)", name))
	}
	return &legacyProviderShim{
		name:    name,
		runner:  runner,
		remover: remover,
		invoker: invoker,
		logger:  logger,
	}
}

var apiProviders = map[string]Provider{
	"digitalocean-functions": mustLegacyProvider("digitalocean-functions", &do.Runner{}, &do.Remover{}, &do.Invoker{}, &do.Logger{}),
	"cloudflare-workers":     mustLegacyProvider("cloudflare-workers", &cf.Runner{}, &cf.Remover{}, &cf.Invoker{}, &cf.Logger{}),
	"vercel":                 mustLegacyProvider("vercel", &vercel.Runner{}, &vercel.Remover{}, &vercel.Invoker{}, &vercel.Logger{}),
	"netlify":                mustLegacyProvider("netlify", &netlify.Runner{}, &netlify.Remover{}, &netlify.Invoker{}, &netlify.Logger{}),
	"fly-machines":           mustLegacyProvider("fly-machines", &fly.Runner{}, &fly.Remover{}, &fly.Invoker{}, &fly.Logger{}),
	"gcp-functions":          mustLegacyProvider("gcp-functions", &gcp.Runner{}, &gcp.Remover{}, &gcp.Invoker{}, &gcp.Logger{}),
	"azure-functions":        mustLegacyProvider("azure-functions", &azure.Runner{}, &azure.Remover{}, &azure.Invoker{}, &azure.Logger{}),
	"kubernetes":             mustLegacyProvider("kubernetes", &k8s.Runner{}, &k8s.Remover{}, &k8s.Invoker{}, &k8s.Logger{}),
	"alibaba-fc":             mustLegacyProvider("alibaba-fc", &alibaba.Runner{}, &alibaba.Remover{}, &alibaba.Invoker{}, &alibaba.Logger{}),
	"ibm-openwhisk":          mustLegacyProvider("ibm-openwhisk", &ibm.Runner{}, &ibm.Remover{}, &ibm.Invoker{}, &ibm.Logger{}),
}

func getProvider(name string) (Provider, bool) {
	p, ok := apiProviders[name]
	return p, ok
}

func hasProvider(name string) bool {
	_, ok := getProvider(name)
	return ok
}

// APIProviderNames returns the list of provider names with API-dispatch support.
// Used by tests, docs sync checks, and resolution boundaries.
func APIProviderNames() []string {
	names := make([]string, 0, len(apiProviders))
	for k := range apiProviders {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
