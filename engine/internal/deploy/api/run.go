// Package api performs real deploys using provider REST APIs and SDKs (no CLI).
// Auth via env vars per provider. Part of internal/deploy; see internal/deploy/cli for CLI-based deploy.
package api

import (
	"context"
	"fmt"
	"sort"

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

// Run deploys via the provider's API and returns a DeployResult. Saves receipt to root.
func Run(ctx context.Context, provider string, cfg *config.Config, stage, root string) (*providers.DeployResult, error) {
	runner, ok := runners[provider]
	if !ok {
		return nil, fmt.Errorf("deploy via API not implemented for provider %q", provider)
	}
	result, err := runner.Deploy(ctx, cfg, stage, root)
	if err != nil {
		return nil, err
	}
	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     result.Provider,
		DeploymentID: result.DeploymentID,
		Outputs:      result.Outputs,
		Artifacts:    result.Artifacts,
		Metadata:     result.Metadata,
		Functions:    make([]state.FunctionDeployment, 0, len(result.Artifacts)),
	}
	for _, a := range result.Artifacts {
		receipt.Functions = append(receipt.Functions, state.FunctionDeployment{Function: a.Function})
	}
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}
	return result, nil
}

// Runner deploys via provider API (no CLI).
type Runner interface {
	Deploy(ctx context.Context, cfg *config.Config, stage, root string) (*providers.DeployResult, error)
}

var runners = map[string]Runner{
	"digitalocean-functions": &do.Runner{},
	"cloudflare-workers":     &cf.Runner{},
	"vercel":                 &vercel.Runner{},
	"netlify":                &netlify.Runner{},
	"fly-machines":           &fly.Runner{},
	"gcp-functions":          &gcp.Runner{},
	"azure-functions":        &azure.Runner{},
	"kubernetes":             &k8s.Runner{},
	"alibaba-fc":             &alibaba.Runner{},
	"ibm-openwhisk":          &ibm.Runner{},
}

// HasRunner returns whether the provider has an API-based deploy runner.
func HasRunner(provider string) bool {
	_, ok := runners[provider]
	return ok
}

// APIProviderNames returns the list of provider names that have an API-based deploy runner.
// Used by tests and doc-sync checks so DEPLOY_PROVIDERS.md stays in sync with code.
func APIProviderNames() []string {
	names := make([]string, 0, len(runners))
	for k := range runners {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
