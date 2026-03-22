package fly

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Logger fetches logs via Fly API (GET /v1/apps/{app}/logs).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	appName := receipt.Metadata["app"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	url := flyAPI + "/apps/" + appName + "/logs"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("FLY_API_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return &providers.LogsResult{Provider: "fly-machines", Function: function, Lines: []string{fmt.Sprintf("Failed to fetch logs: %v", err)}}, nil
	}
	defer resp.Body.Close()
	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	if len(lines) == 0 {
		lines = []string{"No log lines (app may have no recent activity)."}
	}
	return &providers.LogsResult{Provider: "fly-machines", Function: function, Lines: lines}, nil
}
