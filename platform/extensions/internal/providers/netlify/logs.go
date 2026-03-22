package netlify

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

// Logger fetches site logs via Netlify API (GET /sites/{id}/log).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	siteID := receipt.Outputs["site_id"]
	if siteID == "" {
		siteID = receipt.Metadata["site_id"]
	}
	if siteID == "" {
		return &providers.LogsResult{Provider: "netlify", Function: function, Lines: []string{"No site_id in receipt."}}, nil
	}
	url := netlifyAPI + "/sites/" + siteID + "/log"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("NETLIFY_AUTH_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return &providers.LogsResult{
			Provider: "netlify",
			Function: function,
			Lines:    []string{fmt.Sprintf("Logs: %v. Dashboard: https://app.netlify.com/sites/%s/logs", err, siteID)},
		}, nil
	}
	defer resp.Body.Close()
	var lines []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	if len(lines) == 0 {
		lines = []string{"No log entries."}
	}
	return &providers.LogsResult{Provider: "netlify", Function: function, Lines: lines}, nil
}
