package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/runfabric/runfabric/engine/internal/apiutil"
	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

const loggingAPI = "https://logging.googleapis.com/v2/entries:list"

// Logs implements providers.Provider by loading the receipt and delegating to Logger.
// Receipt is loaded from "." (current directory); run from project root so .runfabric/<stage>.json is found.
func (p *Provider) Logs(cfg *providers.Config, stage, function string) (*providers.LogsResult, error) {
	receipt, _ := state.Load(".", stage)
	return (Logger{}).Logs(context.Background(), cfg, stage, function, receipt)
}

// Logger fetches recent log entries from Cloud Logging for the deployed function(s).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	if project == "" || apiutil.Env("GCP_ACCESS_TOKEN") == "" {
		return &providers.LogsResult{
			Provider: "gcp-functions",
			Function: function,
			Lines:    []string{"Set GCP_PROJECT and GCP_ACCESS_TOKEN for live logs; see Cloud Console: https://console.cloud.google.com/functions/list"},
		}, nil
	}

	// Collect function IDs to query: either the requested function or all deployed.
	var funcIDs []string
	if function != "" {
		funcIDs = append(funcIDs, fmt.Sprintf("%s-%s-%s", cfg.Service, stage, function))
	} else {
		for fnName := range cfg.Functions {
			funcIDs = append(funcIDs, fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fnName))
		}
	}

	var allLines []string
	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	for _, funcID := range funcIDs {
		// Gen2 functions use Cloud Run; Gen1 use cloud_function. Try both.
		for _, filter := range []string{
			fmt.Sprintf(`resource.type="cloud_run_revision" AND resource.labels.service_name="%s" AND timestamp>=%q`, funcID, since),
			fmt.Sprintf(`resource.type="cloud_function" AND resource.labels.function_name="%s" AND timestamp>=%q`, funcID, since),
		} {
			lines, err := listLogEntries(ctx, project, filter, 100)
			if err != nil {
				allLines = append(allLines, fmt.Sprintf("[%s] error: %v", funcID, err))
				continue
			}
			if len(lines) > 0 {
				allLines = append(allLines, lines...)
				break
			}
		}
	}

	if len(allLines) == 0 {
		allLines = append(allLines, fmt.Sprintf("No recent logs for stage %s (last 1h). View: https://console.cloud.google.com/functions/list?project=%s", stage, project))
	}
	return &providers.LogsResult{Provider: "gcp-functions", Function: function, Lines: allLines}, nil
}

// listLogEntries POSTs to Cloud Logging API with retry/backoff and returns log lines.
func listLogEntries(ctx context.Context, project, filter string, pageSize int) ([]string, error) {
	var lines []string
	err := apiutil.RetryWithBackoff(ctx, 3, 200*time.Millisecond, func() error {
		l, e := listLogEntriesOnce(ctx, project, filter, pageSize)
		if e != nil {
			return e
		}
		lines = l
		return nil
	})
	return lines, err
}

func listLogEntriesOnce(ctx context.Context, project, filter string, pageSize int) ([]string, error) {
	body := map[string]any{
		"resourceNames": []string{"projects/" + project},
		"filter":        filter,
		"pageSize":      pageSize,
		"orderBy":       "timestamp desc",
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loggingAPI, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiutil.Env("GCP_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiutil.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("logging API %s: %s", resp.Status, string(b))
	}
	var result struct {
		Entries []struct {
			Timestamp   string         `json:"timestamp"`
			Severity    string         `json:"severity"`
			TextPayload string         `json:"textPayload"`
			JsonPayload map[string]any `json:"jsonPayload"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	// Sort by timestamp ascending for chronological display.
	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Timestamp < result.Entries[j].Timestamp
	})
	var lines []string
	for _, e := range result.Entries {
		line := e.TextPayload
		if line == "" && e.JsonPayload != nil {
			j, _ := json.Marshal(e.JsonPayload)
			line = string(j)
		}
		if line == "" {
			line = "(no payload)"
		}
		ts := e.Timestamp
		if len(ts) > 26 {
			ts = ts[:26]
		}
		lines = append(lines, fmt.Sprintf("%s [%s] %s", ts, e.Severity, line))
	}
	return lines, nil
}
