package digitalocean

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

// Logger fetches app logs via DigitalOcean API.
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	appID := receipt.Outputs["app_id"]
	if appID == "" {
		return &providers.LogsResult{
			Provider: "digitalocean-functions",
			Function: function,
			Lines:    []string{"No app_id in receipt. Redeploy then run logs again."},
		}, nil
	}
	url := doAPI + "/" + appID + "/logs?type=run"
	lines, err := getLines(ctx, url, "DIGITALOCEAN_ACCESS_TOKEN")
	if err != nil {
		return &providers.LogsResult{
			Provider: "digitalocean-functions",
			Function: function,
			Lines:    []string{fmt.Sprintf("Fetch logs: %v. Or open https://cloud.digitalocean.com/apps/%s", err, appID)},
		}, nil
	}
	if len(lines) == 0 {
		lines = []string{"No run logs (check Build/Deploy logs in console)."}
	}
	return &providers.LogsResult{Provider: "digitalocean-functions", Function: function, Lines: lines}, nil
}

func getLines(ctx context.Context, url, authEnv string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if t := apiutil.Env(authEnv); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	return lines, sc.Err()
}
