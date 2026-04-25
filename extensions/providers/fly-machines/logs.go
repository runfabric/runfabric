package fly

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Logger fetches logs via Fly API (GET /v1/apps/{app}/logs).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	service := sdkprovider.Service(cfg)
	rv := sdkprovider.DecodeReceipt(receipt)
	appName := rv.Metadata["app"]
	if appName == "" {
		appName = fmt.Sprintf("%s-%s", service, stage)
	}
	url := flyAPI + "/apps/" + appName + "/logs"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("FLY_API_TOKEN"))
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return &sdkprovider.LogsResult{Provider: "fly-machines", Function: function, Lines: []string{fmt.Sprintf("Failed to fetch logs: %v", err)}}, nil
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
	return &sdkprovider.LogsResult{Provider: "fly-machines", Function: function, Lines: lines}, nil
}
