package azure

import (
	"context"
	"fmt"

	coreconfig "github.com/runfabric/runfabric/platform/core/model/config"
	engconfig "github.com/runfabric/runfabric/platform/core/model/config"
	azuretarget "github.com/runfabric/runfabric/platform/extensions/internal/providers/azure"
)

// FunctionMetrics keeps a stable metrics contract between workflow and provider entities.
type FunctionMetrics struct {
	Invocations *float64 `json:"invocations,omitempty"`
	Errors      *float64 `json:"errors,omitempty"`
	DurationAvg *float64 `json:"durationAvgMs,omitempty"`
}

// TraceSummary keeps a stable traces contract between workflow and provider entities.
type TraceSummary struct {
	ID           string  `json:"id,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	ResponseTime float64 `json:"responseTime,omitempty"`
	HTTPStatus   *int32  `json:"httpStatus,omitempty"`
	ServiceCount *int    `json:"serviceCount,omitempty"`
	HasError     *bool   `json:"hasError,omitempty"`
}

func FetchMetrics(ctx context.Context, cfg *coreconfig.Config, stage string) (map[string]FunctionMetrics, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	engCfg := convertCoreConfigToEngineConfig(cfg)

	metrics, err := azuretarget.FetchMetrics(ctx, engCfg, stage)
	if err != nil {
		return nil, err
	}

	result := make(map[string]FunctionMetrics)
	for k, v := range metrics {
		result[k] = FunctionMetrics{
			Invocations: v.Invocations,
			Errors:      v.Errors,
			DurationAvg: v.DurationAvg,
		}
	}
	return result, nil
}

func FetchTraces(ctx context.Context, cfg *coreconfig.Config, stage string) ([]TraceSummary, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	engCfg := convertCoreConfigToEngineConfig(cfg)

	traces, err := azuretarget.FetchTraces(ctx, engCfg, stage)
	if err != nil {
		return nil, err
	}

	result := make([]TraceSummary, len(traces))
	for i, t := range traces {
		result[i] = TraceSummary{
			ID:           t.ID,
			Duration:     t.Duration,
			ResponseTime: t.ResponseTime,
			HTTPStatus:   t.HTTPStatus,
			ServiceCount: t.ServiceCount,
			HasError:     t.HasError,
		}
	}
	return result, nil
}

// Conversion helper
func convertCoreConfigToEngineConfig(cfg *coreconfig.Config) *engconfig.Config {
	return &engconfig.Config{
		Service:    cfg.Service,
		Provider:   cfg.Provider,
		Runtime:    cfg.Runtime,
		Functions:  cfg.Functions,
		Workflows:  cfg.Workflows,
		Secrets:    cfg.Secrets,
		Extensions: cfg.Extensions,
		Addons:     cfg.Addons,
	}
}
