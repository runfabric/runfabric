package app

import (
	"sort"
	"strings"
)

type RouterSimulationResult struct {
	Strategy     string         `json:"strategy"`
	Requests     int            `json:"requests"`
	Down         []string       `json:"down,omitempty"`
	Available    bool           `json:"available"`
	Selected     string         `json:"selected,omitempty"`
	Distribution map[string]int `json:"distribution,omitempty"`
}

type RouterChaosScenario struct {
	Scenario     string         `json:"scenario"`
	Down         []string       `json:"down,omitempty"`
	Available    bool           `json:"available"`
	Selected     string         `json:"selected,omitempty"`
	Distribution map[string]int `json:"distribution,omitempty"`
	Pass         bool           `json:"pass"`
}

type RouterChaosVerification struct {
	Strategy  string                `json:"strategy"`
	Requests  int                   `json:"requests"`
	Scenarios []RouterChaosScenario `json:"scenarios"`
	Pass      bool                  `json:"pass"`
}

// SimulateRouterRouting simulates traffic steering locally without provider API calls.
func SimulateRouterRouting(routingCfg *RouterRoutingConfig, requests int, downProviders []string) RouterSimulationResult {
	if requests <= 0 {
		requests = 100
	}
	result := RouterSimulationResult{
		Requests:     requests,
		Available:    false,
		Distribution: map[string]int{},
	}
	if routingCfg == nil {
		return result
	}
	result.Strategy = routingCfg.Strategy
	result.Down = append([]string(nil), downProviders...)
	sort.Strings(result.Down)
	downSet := toLowerSet(downProviders)
	endpoints := availableEndpoints(routingCfg.Endpoints, downSet)
	if len(endpoints) == 0 {
		return result
	}

	result.Available = true
	switch strings.ToLower(strings.TrimSpace(routingCfg.Strategy)) {
	case "failover":
		sort.Slice(endpoints, func(i, j int) bool {
			if endpoints[i].Priority == endpoints[j].Priority {
				return endpoints[i].Name < endpoints[j].Name
			}
			return endpoints[i].Priority < endpoints[j].Priority
		})
		selected := endpoints[0].Name
		result.Selected = selected
		result.Distribution[selected] = requests
	default:
		for i := 0; i < requests; i++ {
			selected := pickWeightedEndpoint(endpoints, i)
			result.Distribution[selected]++
		}
		result.Selected = pickDominantEndpoint(result.Distribution)
	}
	return result
}

// VerifyRouterFailover simulates one-endpoint-down scenarios plus an all-endpoints-down scenario.
func VerifyRouterFailover(routingCfg *RouterRoutingConfig, requests int) RouterChaosVerification {
	if requests <= 0 {
		requests = 100
	}
	report := RouterChaosVerification{
		Requests: requests,
		Pass:     true,
	}
	if routingCfg == nil {
		report.Pass = false
		return report
	}
	report.Strategy = routingCfg.Strategy

	names := make([]string, 0, len(routingCfg.Endpoints))
	for _, ep := range routingCfg.Endpoints {
		n := strings.TrimSpace(ep.Name)
		if n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, down := range names {
		sim := SimulateRouterRouting(routingCfg, requests, []string{down})
		pass := sim.Available && !strings.EqualFold(strings.TrimSpace(sim.Selected), strings.TrimSpace(down))
		report.Scenarios = append(report.Scenarios, RouterChaosScenario{
			Scenario:     "single-down",
			Down:         []string{down},
			Available:    sim.Available,
			Selected:     sim.Selected,
			Distribution: sim.Distribution,
			Pass:         pass,
		})
		if !pass {
			report.Pass = false
		}
	}

	allDown := SimulateRouterRouting(routingCfg, requests, names)
	allDownPass := !allDown.Available
	report.Scenarios = append(report.Scenarios, RouterChaosScenario{
		Scenario:     "all-down",
		Down:         names,
		Available:    allDown.Available,
		Selected:     allDown.Selected,
		Distribution: allDown.Distribution,
		Pass:         allDownPass,
	})
	if !allDownPass {
		report.Pass = false
	}
	return report
}

func availableEndpoints(endpoints []RouterRoutingEndpoint, downSet map[string]struct{}) []RouterRoutingEndpoint {
	out := make([]RouterRoutingEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		name := strings.ToLower(strings.TrimSpace(ep.Name))
		if _, down := downSet[name]; down {
			continue
		}
		if ep.Healthy != nil && !*ep.Healthy {
			continue
		}
		out = append(out, ep)
	}
	return out
}

func pickWeightedEndpoint(endpoints []RouterRoutingEndpoint, seed int) string {
	total := 0
	for _, ep := range endpoints {
		total += maxInt(ep.Weight, 1)
	}
	if total <= 0 {
		return ""
	}
	slot := seed % total
	cum := 0
	for _, ep := range endpoints {
		cum += maxInt(ep.Weight, 1)
		if slot < cum {
			return ep.Name
		}
	}
	return endpoints[len(endpoints)-1].Name
}

func pickDominantEndpoint(distribution map[string]int) string {
	if len(distribution) == 0 {
		return ""
	}
	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(distribution))
	for k, v := range distribution {
		items = append(items, item{name: k, count: v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].name < items[j].name
		}
		return items[i].count > items[j].count
	})
	return items[0].name
}

func toLowerSet(in []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range in {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}
