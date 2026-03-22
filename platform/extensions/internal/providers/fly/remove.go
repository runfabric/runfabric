package fly

import (
	"context"
	"fmt"
	"net/http"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Remover deletes the app via Fly API (DELETE /v1/apps/{name}?force=true).
type Remover struct{}

func (Remover) Remove(ctx context.Context, cfg *config.Config, stage, root string, receipt *state.Receipt) (*providers.RemoveResult, error) {
	appName := receipt.Metadata["app"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", cfg.Service, stage)
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
	return &providers.RemoveResult{Provider: "fly-machines", Removed: true}, nil
}
