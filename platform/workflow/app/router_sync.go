package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	builtinrouters "github.com/runfabric/runfabric/extensions/routers"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

type RouterSyncResult = builtinrouters.SyncResult

// SelectedRouterPlugin returns extensions.routerPlugin, defaulting to cloudflare.
func SelectedRouterPlugin(cfg *config.Config) string {
	id := strings.ToLower(strings.TrimSpace(config.ExtensionString(cfg, "routerPlugin")))
	if id == "" {
		return "cloudflare"
	}
	return id
}

// RouterDNSSync dispatches router DNS/LB sync through the configured router plugin.
func RouterDNSSync(ctx *AppContext, routing *RouterRoutingConfig, zoneID, accountID string, dryRun bool, out io.Writer) (*RouterSyncResult, error) {
	if ctx == nil || ctx.Extensions == nil {
		return nil, fmt.Errorf("app context extensions are not initialized")
	}
	pluginID := SelectedRouterPlugin(ctx.Config)
	return ctx.Extensions.SyncRouter(context.Background(), pluginID, RouterSyncRequest{
		Routing:   routing,
		ZoneID:    zoneID,
		AccountID: accountID,
		DryRun:    dryRun,
		Out:       out,
	})
}
