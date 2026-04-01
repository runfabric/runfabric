package route53

import (
	"context"
	"io"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// TransportPlugin wraps the Route53 router for external stdio serving.
type TransportPlugin struct {
	router sdkrouter.Router
}

func NewTransportPlugin() *TransportPlugin {
	return &TransportPlugin{router: NewRouter()}
}

func (p *TransportPlugin) Meta() sdkrouter.PluginMeta {
	return RouterMeta()
}

func (p *TransportPlugin) Sync(ctx context.Context, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	req.Out = io.Discard
	return p.router.Sync(ctx, req)
}
