package azure

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

type durableFunctionDecl struct {
	Name                     string
	Orchestrator             string
	TaskHub                  string
	StorageConnectionSetting string
}

func durableFunctionsFromConfig(cfg *config.Config) ([]durableFunctionDecl, error) {
	if cfg == nil || cfg.Extensions == nil {
		return nil, nil
	}
	rawAzure, ok := cfg.Extensions["azure-functions"]
	if !ok || rawAzure == nil {
		return nil, nil
	}
	azureMap, ok := rawAzure.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("extensions.azure-functions must be an object")
	}
	rawDurable, ok := azureMap["durableFunctions"]
	if !ok || rawDurable == nil {
		return nil, nil
	}
	items, ok := rawDurable.([]any)
	if !ok {
		return nil, fmt.Errorf("extensions.azure-functions.durableFunctions must be an array")
	}
	out := make([]durableFunctionDecl, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("extensions.azure-functions.durableFunctions[%d] must be an object", i)
		}
		name := strings.TrimSpace(anyString(m["name"]))
		if name == "" {
			return nil, fmt.Errorf("extensions.azure-functions.durableFunctions[%d].name is required", i)
		}
		orchestrator := strings.TrimSpace(anyString(m["orchestrator"]))
		if orchestrator == "" {
			orchestrator = name
		}
		taskHub := strings.TrimSpace(anyString(m["taskHub"]))
		storageConnectionSetting := strings.TrimSpace(anyString(m["storageConnectionSetting"]))
		out = append(out, durableFunctionDecl{
			Name:                     name,
			Orchestrator:             orchestrator,
			TaskHub:                  taskHub,
			StorageConnectionSetting: storageConnectionSetting,
		})
	}
	return out, nil
}

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
