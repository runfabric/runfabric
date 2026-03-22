package vercel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	"github.com/runfabric/runfabric/platform/core/model/config"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// Logger fetches deployment events via Vercel API (GET /v3/deployments/{id}/events).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	deployID := receipt.Outputs["deployment_id"]
	if deployID == "" {
		return &providers.LogsResult{
			Provider: "vercel",
			Function: function,
			Lines:    []string{"No deployment_id in receipt; redeploy to capture logs."},
		}, nil
	}
	url := vercelAPI + "/v3/deployments/" + deployID + "/events?limit=50"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("VERCEL_TOKEN"))
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return &providers.LogsResult{Provider: "vercel", Function: function, Lines: []string{err.Error()}}, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var events []struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if json.Unmarshal(b, &events) != nil {
		lines := []string{string(b)}
		if len(lines[0]) > 500 {
			lines[0] = lines[0][:500] + "..."
		}
		return &providers.LogsResult{Provider: "vercel", Function: function, Lines: lines}, nil
	}
	var lines []string
	for _, e := range events {
		lines = append(lines, fmt.Sprintf("[%s] %v", e.Type, e.Payload))
	}
	if len(lines) == 0 {
		lines = []string{"No deployment events."}
	}
	return &providers.LogsResult{Provider: "vercel", Function: function, Lines: lines}, nil
}
