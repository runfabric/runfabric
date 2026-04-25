package app

import (
	"math"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func applyRouterQualityScoring(routingCfg *RouterRoutingConfig, cfg *config.Config) {
	if routingCfg == nil || len(routingCfg.Endpoints) == 0 {
		return
	}
	routerObj := routerExtensionObject(cfg)
	qualityObj, ok := routerAsMap(routerObj["qualityScoring"])
	if !ok || !routerAsBool(qualityObj["enabled"], false) {
		return
	}

	unhealthyPenalty := clampInt(routerAsInt(qualityObj["unhealthyPenaltyPercent"], 80), 0, 100)
	providerMultiplier := map[string]int{}
	if raw, ok := routerAsMap(qualityObj["providerMultiplier"]); ok {
		for k, v := range raw {
			key := strings.ToLower(strings.TrimSpace(k))
			if key == "" {
				continue
			}
			providerMultiplier[key] = clampInt(routerAsInt(v, 100), 1, 500)
		}
	}

	for i := range routingCfg.Endpoints {
		ep := &routingCfg.Endpoints[i]
		weight := maxInt(ep.Weight, 100)
		if ep.Healthy != nil && !*ep.Healthy {
			weight = maxInt(1, (weight*(100-unhealthyPenalty))/100)
		}
		if m, ok := providerMultiplier[strings.ToLower(strings.TrimSpace(ep.Name))]; ok {
			weight = maxInt(1, int(math.Round(float64(weight)*float64(m)/100.0)))
		}
		ep.Weight = weight
	}
	rebalanceEndpointWeights(routingCfg.Endpoints, 100)
}

func applyRouterCanaryDefaults(routingCfg *RouterRoutingConfig, cfg *config.Config) {
	if routingCfg == nil || len(routingCfg.Endpoints) < 2 {
		return
	}
	routerObj := routerExtensionObject(cfg)
	canaryObj, ok := routerAsMap(routerObj["canary"])
	if !ok || !routerAsBool(canaryObj["enabled"], false) {
		return
	}
	provider := strings.TrimSpace(routerAsString(canaryObj["provider"], ""))
	if provider == "" {
		return
	}
	percent := clampInt(routerAsInt(canaryObj["percent"], 10), 0, 100)
	ApplyCanaryWeights(routingCfg, provider, percent)
}

// ApplyCanaryWeights sets endpoint weights so traffic can be shifted progressively to one provider.
func ApplyCanaryWeights(routingCfg *RouterRoutingConfig, provider string, percent int) bool {
	if routingCfg == nil || len(routingCfg.Endpoints) < 2 {
		return false
	}
	targetProvider := strings.ToLower(strings.TrimSpace(provider))
	if targetProvider == "" {
		return false
	}

	canaryIdx := -1
	for i := range routingCfg.Endpoints {
		if strings.EqualFold(strings.TrimSpace(routingCfg.Endpoints[i].Name), targetProvider) {
			canaryIdx = i
			break
		}
	}
	if canaryIdx < 0 {
		return false
	}

	percent = clampInt(percent, 0, 100)
	if percent == 0 {
		percent = 1
	}
	if percent >= 100 {
		percent = 99
	}

	otherCount := len(routingCfg.Endpoints) - 1
	otherShare := 100 - percent
	if otherShare < otherCount {
		otherShare = otherCount
		percent = 100 - otherShare
	}
	if percent < 1 {
		percent = 1
	}

	baseSum := 0
	for i, ep := range routingCfg.Endpoints {
		if i == canaryIdx {
			continue
		}
		baseSum += maxInt(ep.Weight, 1)
	}
	if baseSum <= 0 {
		baseSum = otherCount
	}

	for i := range routingCfg.Endpoints {
		if i == canaryIdx {
			routingCfg.Endpoints[i].Weight = percent
			continue
		}
		base := maxInt(routingCfg.Endpoints[i].Weight, 1)
		scaled := int(math.Round(float64(base) * float64(otherShare) / float64(baseSum)))
		routingCfg.Endpoints[i].Weight = maxInt(1, scaled)
	}
	rebalanceEndpointWeights(routingCfg.Endpoints, 100)
	return true
}

func rebalanceEndpointWeights(endpoints []RouterRoutingEndpoint, target int) {
	if len(endpoints) == 0 || target <= 0 {
		return
	}
	baseSum := 0
	for i := range endpoints {
		if endpoints[i].Weight <= 0 {
			endpoints[i].Weight = 1
		}
		baseSum += endpoints[i].Weight
	}
	if baseSum <= 0 {
		equal := maxInt(1, target/len(endpoints))
		for i := range endpoints {
			endpoints[i].Weight = equal
		}
		baseSum = equal * len(endpoints)
	}

	if baseSum != target {
		scale := float64(target) / float64(baseSum)
		for i := range endpoints {
			endpoints[i].Weight = maxInt(1, int(math.Round(float64(endpoints[i].Weight)*scale)))
		}
	}

	sum := 0
	for i := range endpoints {
		sum += endpoints[i].Weight
	}
	delta := target - sum
	if delta == 0 {
		return
	}
	attempts := 0
	maxAttempts := len(endpoints) * 4
	for i := 0; delta != 0 && len(endpoints) > 0; i = (i + 1) % len(endpoints) {
		if delta > 0 {
			endpoints[i].Weight++
			delta--
			attempts = 0
			continue
		}
		if endpoints[i].Weight > 1 {
			endpoints[i].Weight--
			delta++
			attempts = 0
			continue
		}
		attempts++
		if attempts >= maxAttempts {
			break
		}
	}
}

// routerHostname returns extensions.router.hostname when set, else empty string.
func routerHostname(cfg *config.Config) string {
	obj := routerExtensionObject(cfg)
	if v, ok := obj["hostname"].(string); ok {
		return v
	}
	return ""
}

func routerExtensionObject(cfg *config.Config) map[string]any {
	if cfg == nil || cfg.Extensions == nil {
		return map[string]any{}
	}
	routerObj, ok := routerAsMap(cfg.Extensions["router"])
	if !ok {
		return map[string]any{}
	}
	return routerObj
}

func routerAsMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	out, ok := v.(map[string]any)
	return out, ok
}

func routerAsBool(v any, fallback bool) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		if s == "true" {
			return true
		}
		if s == "false" {
			return false
		}
	}
	return fallback
}

func routerAsInt(v any, fallback int) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	case float32:
		return int(x)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return fallback
		}
		sign := 1
		if strings.HasPrefix(s, "-") {
			sign = -1
			s = strings.TrimPrefix(s, "-")
		}
		n := 0
		for _, r := range s {
			if r < '0' || r > '9' {
				return fallback
			}
			n = n*10 + int(r-'0')
		}
		return sign * n
	}
	return fallback
}

func routerAsString(v any, fallback string) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return fallback
	}
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
