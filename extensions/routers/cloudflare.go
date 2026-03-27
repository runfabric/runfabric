package routers

import (
	"context"
	"os"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// CloudflareRouter implements sdkrouter.Router for Cloudflare DNS/LB sync.
type CloudflareRouter struct{}

// NewCloudflareRouter returns a new CloudflareRouter.
func NewCloudflareRouter() sdkrouter.Router {
	return CloudflareRouter{}
}

func (CloudflareRouter) Meta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{
		ID:          "cloudflare",
		Name:        "Cloudflare Router",
		Description: "Cloudflare DNS/LB router sync plugin",
	}
}

func (CloudflareRouter) Sync(ctx context.Context, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	s, err := newCloudflareSyncer(cloudflareConfig{
		APIToken:  os.Getenv("CLOUDFLARE_API_TOKEN"),
		ZoneID:    req.ZoneID,
		AccountID: req.AccountID,
	}, req.DryRun, req.Out)
	if err != nil {
		return nil, err
	}
	return s.sync(ctx, req.Routing)
}

func BuiltinRouterManifests() []Meta {
	return []Meta{{ID: "cloudflare", Name: "Cloudflare Router", Description: "Cloudflare DNS/LB router sync plugin"}}
}

func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(NewCloudflareRouter())
	return reg
}
