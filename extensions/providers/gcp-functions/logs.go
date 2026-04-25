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

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

const loggingAPI = "https://logging.googleapis.com/v2/entries:list"

// Logger fetches recent log entries from Cloud Logging for the deployed function(s).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	service := strings.TrimSpace(sdkprovider.Service(cfg))
	functions := sdkprovider.Functions(cfg)
	project := sdkprovider.Env("GCP_PROJECT")
	if project == "" {
		project = sdkprovider.Env("GCP_PROJECT_ID")
	}
	if project == "" || sdkprovider.Env("GCP_ACCESS_TOKEN") == "" {
		return &sdkprovider.LogsResult{
			Provider: "gcp-functions",
			Function: function,
			Lines:    []string{"Set GCP_PROJECT and GCP_ACCESS_TOKEN for live logs; see Cloud Console: https://console.cloud.google.com/functions/list"},
		}, nil
	}
	if service == "" {
		return &sdkprovider.LogsResult{Provider: "gcp-functions", Function: function, Lines: []string{"No config available"}}, nil
	}

	// Collect function IDs to query: either the requested function or all deployed.
	var funcIDs []string
	if function != "" {
		funcIDs = append(funcIDs, fmt.Sprintf("%s-%s-%s", service, stage, function))
	} else {
		for fnName := range functions {
			funcIDs = append(funcIDs, fmt.Sprintf("%s-%s-%s", service, stage, fnName))
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
	return &sdkprovider.LogsResult{Provider: "gcp-functions", Function: function, Lines: allLines}, nil
}

// listLogEntries POSTs to Cloud Logging API with retry/backoff and returns log lines.
func listLogEntries(ctx context.Context, project, filter string, pageSize int) ([]string, error) {
	var lines []string
	err := retryWithBackoff(ctx, 3, 200*time.Millisecond, func() error {
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
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("GCP_ACCESS_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sdkprovider.DefaultClient.Do(req)
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
