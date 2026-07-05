package cloudflare

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// Router implements sdkrouter.Router for Cloudflare DNS/LB sync.
type Router struct{}

// NewRouter returns a new Cloudflare router reconciler.
func NewRouter() sdkrouter.Router {
	return Router{}
}

func RouterMeta() sdkrouter.PluginMeta {
	return sdkrouter.PluginMeta{
		ID:          "cloudflare",
		Name:        "Cloudflare Router",
		Description: "Cloudflare DNS/LB router sync plugin",
	}
}

func (Router) Meta() sdkrouter.PluginMeta {
	return RouterMeta()
}

func (Router) Sync(ctx context.Context, req sdkrouter.RouterSyncRequest) (*sdkrouter.RouterSyncResult, error) {
	// Request values win; env (router keys, then declared same-cloud provider
	// fallbacks) fills the gaps.
	zoneID := strings.TrimSpace(req.ZoneID)
	if zoneID == "" {
		zoneID = sdkprovider.ResolveVar(CredentialVars, "RUNFABRIC_ROUTER_ZONE_ID")
	}
	accountID := strings.TrimSpace(req.AccountID)
	if accountID == "" {
		accountID = sdkprovider.ResolveVar(CredentialVars, "RUNFABRIC_ROUTER_ACCOUNT_ID")
	}
	s, err := newCloudflareSyncer(cloudflareConfig{
		APIToken:  resolveCloudflareAPIToken(),
		ZoneID:    zoneID,
		AccountID: accountID,
	}, req.DryRun, req.Out)
	if err != nil {
		return nil, err
	}
	return s.sync(ctx, req.Routing)
}

// resolveCloudflareAPIToken: router token env → token file → declared
// fallback (the cloudflare-workers provider's CLOUDFLARE_API_TOKEN).
func resolveCloudflareAPIToken() string {
	if value := strings.TrimSpace(os.Getenv("RUNFABRIC_ROUTER_API_TOKEN")); value != "" {
		return value
	}
	if value := readCloudflareAPITokenFile("RUNFABRIC_ROUTER_API_TOKEN_FILE"); value != "" {
		return value
	}
	return sdkprovider.ResolveVar(CredentialVars, "RUNFABRIC_ROUTER_API_TOKEN")
}

func readCloudflareAPITokenFile(envKey string) string {
	path := strings.TrimSpace(os.Getenv(envKey))
	if path == "" {
		return ""
	}
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
