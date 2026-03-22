package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	coredevstream "github.com/runfabric/runfabric/platform/core/model/devstream"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// DevStreamState holds state for redirecting Cloudflare Workers to a tunnel and restoring on exit.
type DevStreamState struct {
	AccountID      string
	ZoneID         string
	WorkerName     string
	ProxyWorker    string
	RouteRestore   []cfWorkerRoute
	CreatedRoutes  []cfWorkerRoute
	Applied        bool
	EffectiveMode  coredevstream.Mode
	MissingPrereqs []string
	StatusMessage  string
}

type cfWorkerRoute struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
	Script  string `json:"script"`
}

type cfRoutesResponse struct {
	Success bool            `json:"success"`
	Result  []cfWorkerRoute `json:"result"`
}

type cfRouteResponse struct {
	Success bool          `json:"success"`
	Result  cfWorkerRoute `json:"result"`
}

// RedirectToTunnel finds the Cloudflare Worker for the service/stage and sets up tunnel redirection.
// Cloudflare Workers don't support direct routing override like AWS API Gateway.
// Instead, this returns a state that developers can use to configure environment variables
// or deploy a routing worker that proxies to the tunnel.
// Call Restore to revert.
func RedirectToTunnel(ctx context.Context, cfg *config.Config, stage, tunnelURL string) (*DevStreamState, error) {
	if cfg == nil || stage == "" || tunnelURL == "" {
		return nil, fmt.Errorf("config, stage, and tunnel URL required")
	}
	accountID := apiutil.Env("CLOUDFLARE_ACCOUNT_ID")
	zoneID := apiutil.Env("CLOUDFLARE_ZONE_ID")
	token := apiutil.Env("CLOUDFLARE_API_TOKEN")

	workerName := fmt.Sprintf("%s-%s", cfg.Service, stage)
	proxyWorker := fmt.Sprintf("%s-devstream-proxy", workerName)

	state := &DevStreamState{
		AccountID:   accountID,
		ZoneID:      zoneID,
		WorkerName:  workerName,
		ProxyWorker: proxyWorker,
	}
	status := coredevstream.EvaluateProvider("cloudflare-workers")
	state.EffectiveMode = status.EffectiveMode
	state.MissingPrereqs = append([]string(nil), status.MissingPrereqs...)
	state.StatusMessage = status.Message

	// Missing prerequisites => lifecycle hook only (no cloud mutation).
	if len(state.MissingPrereqs) > 0 || accountID == "" || token == "" {
		return state, nil
	}

	var routesResp cfRoutesResponse
	routesURL := cfAPI + "/zones/" + zoneID + "/workers/routes"
	if err := apiutil.APIGet(ctx, routesURL, "CLOUDFLARE_API_TOKEN", &routesResp); err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: route lookup failed: %v", err)
		return state, nil
	}

	for _, route := range routesResp.Result {
		if route.Script == workerName && route.ID != "" && route.Pattern != "" {
			state.RouteRestore = append(state.RouteRestore, route)
		}
	}

	proxyCode := buildTunnelProxyScript(tunnelURL)
	putURL := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + proxyWorker
	if _, err := apiutil.APIPut(ctx, putURL, "CLOUDFLARE_API_TOKEN", []byte(proxyCode), "application/javascript"); err != nil {
		state.EffectiveMode = coredevstream.ModeLifecycleOnly
		state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: could not create proxy worker: %v", err)
		return state, nil
	}
	state.Applied = true
	if len(state.RouteRestore) == 0 {
		pattern := resolveDevRoutePattern(cfg, stage)
		if pattern == "" {
			_ = state.Restore(ctx, "")
			state.Applied = false
			state.EffectiveMode = coredevstream.ModeLifecycleOnly
			state.StatusMessage = "provider-side mutation skipped: no matching Cloudflare routes were found and no fallback pattern is configured (set stages.<stage>.http.domain.name or CLOUDFLARE_DEV_ROUTE_PATTERN)"
			return state, nil
		}
		createURL := cfAPI + "/zones/" + zoneID + "/workers/routes"
		payload := map[string]string{"pattern": pattern, "script": proxyWorker}
		var created cfRouteResponse
		if err := apiutil.APIPost(ctx, createURL, "CLOUDFLARE_API_TOKEN", payload, &created); err != nil {
			_ = state.Restore(ctx, "")
			state.Applied = false
			state.EffectiveMode = coredevstream.ModeLifecycleOnly
			state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: could not create fallback route %q: %v", pattern, err)
			return state, nil
		}
		if created.Result.ID == "" {
			created.Result.ID = "created-without-id"
		}
		if created.Result.Pattern == "" {
			created.Result.Pattern = pattern
		}
		if created.Result.Script == "" {
			created.Result.Script = proxyWorker
		}
		state.CreatedRoutes = append(state.CreatedRoutes, created.Result)
	}

	for _, route := range state.RouteRestore {
		updateURL := cfAPI + "/zones/" + zoneID + "/workers/routes/" + route.ID
		payload := map[string]string{"pattern": route.Pattern, "script": proxyWorker}
		body, _ := json.Marshal(payload)
		if _, err := apiutil.APIPut(ctx, updateURL, "CLOUDFLARE_API_TOKEN", body, "application/json"); err != nil {
			_ = state.Restore(ctx, "")
			state.Applied = false
			state.EffectiveMode = coredevstream.ModeLifecycleOnly
			state.StatusMessage = fmt.Sprintf("provider-side mutation skipped: route update failed for %s: %v", route.Pattern, err)
			return state, nil
		}
	}
	state.EffectiveMode = coredevstream.ModeRouteRewrite
	if len(state.CreatedRoutes) > 0 {
		state.StatusMessage = "full route rewrite applied by creating a temporary Cloudflare route that points to a tunnel proxy worker; the temporary route and proxy worker will be removed on exit"
	} else {
		state.StatusMessage = "full route rewrite applied by pointing Cloudflare routes to a temporary tunnel proxy worker; original routes will be restored on exit"
	}

	return state, nil
}

// Restore reverts the worker to its original configuration.
func (s *DevStreamState) Restore(ctx context.Context, accountID string) error {
	if s == nil || s.WorkerName == "" {
		return nil
	}
	if !s.Applied {
		return nil
	}
	if s.ZoneID == "" {
		s.ZoneID = apiutil.Env("CLOUDFLARE_ZONE_ID")
	}
	if accountID == "" {
		accountID = s.AccountID
	}
	if accountID == "" || apiutil.Env("CLOUDFLARE_API_TOKEN") == "" {
		return nil
	}
	var errs []error

	for _, route := range s.RouteRestore {
		if route.ID == "" || route.Pattern == "" {
			continue
		}
		script := route.Script
		if script == "" {
			script = s.WorkerName
		}
		updateURL := cfAPI + "/zones/" + s.ZoneID + "/workers/routes/" + route.ID
		payload := map[string]string{"pattern": route.Pattern, "script": script}
		body, _ := json.Marshal(payload)
		if _, err := apiutil.APIPut(ctx, updateURL, "CLOUDFLARE_API_TOKEN", body, "application/json"); err != nil {
			errs = append(errs, fmt.Errorf("restore route %s (%s): %w", route.ID, route.Pattern, err))
		}
	}

	for _, route := range s.CreatedRoutes {
		if route.ID == "" || route.ID == "created-without-id" {
			continue
		}
		deleteRouteURL := cfAPI + "/zones/" + s.ZoneID + "/workers/routes/" + route.ID
		if err := apiutil.DoDelete(ctx, deleteRouteURL, "CLOUDFLARE_API_TOKEN"); err != nil {
			errs = append(errs, fmt.Errorf("delete created route %s (%s): %w", route.ID, route.Pattern, err))
		}
	}

	deleteURL := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + s.ProxyWorker
	if err := apiutil.DoDelete(ctx, deleteURL, "CLOUDFLARE_API_TOKEN"); err != nil {
		errs = append(errs, fmt.Errorf("delete proxy worker %s: %w", s.ProxyWorker, err))
	}

	s.Applied = false
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func resolveDevRoutePattern(cfg *config.Config, stage string) string {
	if v := strings.TrimSpace(apiutil.Env("CLOUDFLARE_DEV_ROUTE_PATTERN")); v != "" {
		return v
	}
	if cfg == nil {
		return ""
	}
	if sc, ok := cfg.Stages[stage]; ok && sc.HTTP != nil && sc.HTTP.Domain != nil {
		domain := strings.TrimSpace(sc.HTTP.Domain.Name)
		if domain != "" {
			return domain + "/*"
		}
	}
	return ""
}

func buildTunnelProxyScript(tunnelURL string) string {
	escaped := strings.ReplaceAll(tunnelURL, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return "export default {\\n" +
		"  async fetch(request) {\\n" +
		"    const base = \"" + escaped + "\";\\n" +
		"    const incoming = new URL(request.url);\\n" +
		"    const target = new URL(base);\\n" +
		"    target.pathname = incoming.pathname;\\n" +
		"    target.search = incoming.search;\\n" +
		"    return fetch(target.toString(), request);\\n" +
		"  }\\n" +
		"};\\n"
}
