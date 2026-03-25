package fly

import (
	"context"
	"fmt"
	"net/http"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Remover deletes the app via Fly API (DELETE /v1/apps/{name}?force=true).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg sdkprovider.Config, stage, root string, receipt any) (*sdkprovider.RemoveResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	appName := rv.Metadata["app"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", coreCfg.Service, stage)
	}
	url := flyAPI + "/apps/" + appName + "?force=true"
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("FLY_API_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("fly delete app: %s", resp.Status)
	}
	return &sdkprovider.RemoveResult{Provider: "fly-machines", Removed: true}, nil
}
