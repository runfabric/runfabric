package app

import (
	"fmt"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	"github.com/runfabric/runfabric/platform/workflow/lifecycle"
)

// ProviderDoctor runs provider doctor checks for the selected provider.
// If providerName is empty, the provider from config is used.
func ProviderDoctor(configPath, stage, providerOverride, providerName string) (*providers.DoctorResult, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}

	selected := strings.TrimSpace(providerName)
	if selected == "" {
		selected = strings.TrimSpace(ctx.Config.Provider.Name)
	}
	if selected == "" {
		return nil, fmt.Errorf("provider name is empty")
	}
	if _, ok := ctx.Registry.Get(selected); !ok {
		return nil, fmt.Errorf("plugin %q not found", selected)
	}

	cfg := *ctx.Config
	cfg.Provider.Name = selected
	return lifecycle.Doctor(ctx.Registry, &cfg, ctx.Stage)
}
