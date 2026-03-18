package api

import (
	"context"
	"fmt"

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

// Logger fetches logs via provider API.
type Logger interface {
	Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error)
}

var loggers = map[string]Logger{
	"digitalocean-functions": &do.Logger{},
	"cloudflare-workers":     &cf.Logger{},
	"vercel":                 &vercel.Logger{},
	"netlify":                &netlify.Logger{},
	"fly-machines":           &fly.Logger{},
	"gcp-functions":          &gcp.Logger{},
	"azure-functions":        &azure.Logger{},
	"kubernetes":             &k8s.Logger{},
	"alibaba-fc":             &alibaba.Logger{},
	"ibm-openwhisk":          &ibm.Logger{},
}

// Logs returns logs for the deployed function via provider API.
// If receipt is nil, receipt is loaded from state.Load(root, stage) for backward compatibility.
func Logs(ctx context.Context, provider string, cfg *config.Config, stage, function string, root string, receipt *state.Receipt) (*providers.LogsResult, error) {
	logger, ok := loggers[provider]
	if !ok {
		return nil, fmt.Errorf("logs via API not implemented for provider %q", provider)
	}
	if receipt == nil {
		var err error
		receipt, err = state.Load(root, stage)
		if err != nil {
			return nil, fmt.Errorf("no deployment found for stage %q (run deploy first): %w", stage, err)
		}
	}
	if receipt.Provider != provider {
		return nil, fmt.Errorf("receipt provider %q does not match %q", receipt.Provider, provider)
	}
	return logger.Logs(ctx, cfg, stage, function, receipt)
}

// HasLogger returns whether the provider has an API-based logger.
func HasLogger(provider string) bool {
	_, ok := loggers[provider]
	return ok
}
