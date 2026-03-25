package netlify

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	"github.com/runfabric/runfabric/platform/extensions/sdkbridge"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Logger fetches site logs via Netlify API (GET /sites/{id}/log).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	coreCfg, err := sdkbridge.ToCoreConfig(cfg)
	if err != nil {
		return nil, err
	}
	_ = coreCfg
	rv := apiutil.DecodeReceipt(receipt)
	siteID := rv.Outputs["site_id"]
	if siteID == "" {
		siteID = rv.Metadata["site_id"]
	}
	if siteID == "" {
		return &sdkprovider.LogsResult{Provider: "netlify", Function: function, Lines: []string{"No site_id in rv."}}, nil
	}
	url := netlifyAPI + "/sites/" + siteID + "/log"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("NETLIFY_AUTH_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return &sdkprovider.LogsResult{
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
	return &sdkprovider.LogsResult{Provider: "netlify", Function: function, Lines: lines}, nil
}
