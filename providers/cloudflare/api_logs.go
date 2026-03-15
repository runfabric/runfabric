package cloudflare

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Logger fetches Worker logs (tail API or dashboard link).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	accountID := apiutil.Env("CLOUDFLARE_ACCOUNT_ID")
	workerName := receipt.Metadata["worker"]
	if workerName == "" {
		workerName = fmt.Sprintf("%s-%s", cfg.Service, stage)
	}
	url := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + workerName + "/tail"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("CLOUDFLARE_API_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return &providers.LogsResult{
			Provider: "cloudflare-workers",
			Function: function,
			Lines:    []string{fmt.Sprintf("Logs: %v. Dashboard: https://dash.cloudflare.com/ (Workers)", err)},
		}, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return &providers.LogsResult{
			Provider: "cloudflare-workers",
			Function: function,
			Lines:    []string{fmt.Sprintf("Logs API: %s. Dashboard: https://dash.cloudflare.com/ (Workers)", string(b))},
		}, nil
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{"No tail output. Use dashboard for Real-time Logs."}
	}
	return &providers.LogsResult{Provider: "cloudflare-workers", Function: function, Lines: lines}, nil
}
