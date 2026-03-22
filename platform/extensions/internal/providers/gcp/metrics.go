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

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// FunctionMetrics holds per-function metrics (compatible shape with AWS Lambda metrics for CLI output).
type FunctionMetrics struct {
	Invocations *float64 `json:"invocations,omitempty"`
	Errors      *float64 `json:"errors,omitempty"`
	DurationAvg *float64 `json:"durationAvgMs,omitempty"`
}

const gcpLoggingEntriesAPI = "https://logging.googleapis.com/v2/entries:list"

type gcpLogEntry struct {
	Timestamp   string         `json:"timestamp"`
	Severity    string         `json:"severity"`
	TextPayload string         `json:"textPayload"`
	JSONPayload map[string]any `json:"jsonPayload"`
}

// FetchMetrics returns per-function metrics from Cloud Logging over the last hour.
// It approximates invocations as log entry count and errors as ERROR+ severity count.
func FetchMetrics(ctx context.Context, cfg *config.Config, stage string) (map[string]FunctionMetrics, error) {
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	if project == "" || strings.TrimSpace(apiutil.Env("GCP_ACCESS_TOKEN")) == "" {
		return map[string]FunctionMetrics{}, nil
	}

	out := make(map[string]FunctionMetrics)
	for fnName := range cfg.Functions {
		funcID := fmt.Sprintf("%s-%s-%s", cfg.Service, stage, fnName)
		entries, err := gcpFetchFunctionLogEntries(ctx, project, funcID)
		if err != nil {
			continue
		}

		if len(entries) == 0 {
			continue
		}
		invocations := float64(len(entries))
		errors := float64(0)
		for _, e := range entries {
			if gcpSeverityIsError(e.Severity) {
				errors++
			}
		}
		inv := invocations
		errCount := errors
		out[fnName] = FunctionMetrics{
			Invocations: &inv,
			Errors:      &errCount,
			DurationAvg: nil,
		}
	}

	return out, nil
}

func gcpFetchFunctionLogEntries(ctx context.Context, project, functionID string) ([]gcpLogEntry, error) {
	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	filters := []string{
		fmt.Sprintf(`resource.type="cloud_run_revision" AND resource.labels.service_name="%s" AND timestamp>=%q`, functionID, since),
		fmt.Sprintf(`resource.type="cloud_function" AND resource.labels.function_name="%s" AND timestamp>=%q`, functionID, since),
	}

	for _, filter := range filters {
		entries, err := gcpListEntries(ctx, project, filter, 300)
		if err != nil {
			continue
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}
	return []gcpLogEntry{}, nil
}

func gcpListEntries(ctx context.Context, project, filter string, pageSize int) ([]gcpLogEntry, error) {
	var result []gcpLogEntry
	err := apiutil.RetryWithBackoff(ctx, 3, 200*time.Millisecond, func() error {
		entries, e := gcpListEntriesOnce(ctx, project, filter, pageSize)
		if e != nil {
			return e
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func gcpListEntriesOnce(ctx context.Context, project, filter string, pageSize int) ([]gcpLogEntry, error) {
	body := map[string]any{
		"resourceNames": []string{"projects/" + project},
		"filter":        filter,
		"pageSize":      pageSize,
		"orderBy":       "timestamp desc",
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gcpLoggingEntriesAPI, strings.NewReader(string(bodyBytes)))
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
	var payload struct {
		Entries []gcpLogEntry `json:"entries"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	sort.Slice(payload.Entries, func(i, j int) bool {
		return payload.Entries[i].Timestamp < payload.Entries[j].Timestamp
	})
	return payload.Entries, nil
}

func gcpSeverityIsError(severity string) bool {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "ERROR", "CRITICAL", "ALERT", "EMERGENCY":
		return true
	default:
		return false
	}
}
