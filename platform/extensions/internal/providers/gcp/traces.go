package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
)

// TraceSummary is a simplified trace summary for CLI output (compatible shape with AWS X-Ray).
type TraceSummary struct {
	ID           string  `json:"id,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	ResponseTime float64 `json:"responseTime,omitempty"`
	HTTPStatus   *int32  `json:"httpStatus,omitempty"`
	ServiceCount *int    `json:"serviceCount,omitempty"`
	HasError     *bool   `json:"hasError,omitempty"`
}

const gcpTraceAPIBase = "https://cloudtrace.googleapis.com/v1/projects"

type gcpTraceSpan struct {
	StartTime string            `json:"startTime"`
	EndTime   string            `json:"endTime"`
	Labels    map[string]string `json:"labels"`
}

type gcpTrace struct {
	TraceID string         `json:"traceId"`
	Spans   []gcpTraceSpan `json:"spans"`
}

// FetchTraces returns trace summaries from Cloud Trace for the last hour.
func FetchTraces(ctx context.Context, cfg *config.Config, stage string) ([]TraceSummary, error) {
	_ = stage
	project := apiutil.Env("GCP_PROJECT")
	if project == "" {
		project = apiutil.Env("GCP_PROJECT_ID")
	}
	if project == "" || strings.TrimSpace(apiutil.Env("GCP_ACCESS_TOKEN")) == "" {
		return []TraceSummary{}, nil
	}

	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339Nano)
	query := url.Values{}
	query.Set("pageSize", "50")
	query.Set("startTime", since)

	endpoint := fmt.Sprintf("%s/%s/traces?%s", gcpTraceAPIBase, project, query.Encode())
	var payload struct {
		Traces []gcpTrace `json:"traces"`
	}
	if err := apiutil.APIGet(ctx, endpoint, "GCP_ACCESS_TOKEN", &payload); err != nil {
		return nil, err
	}

	out := make([]TraceSummary, 0, len(payload.Traces))
	for _, tr := range payload.Traces {
		summary := gcpTraceToSummary(tr, cfg.Service)
		if summary != nil {
			out = append(out, *summary)
		}
	}
	return out, nil
}

func gcpTraceToSummary(tr gcpTrace, service string) *TraceSummary {
	if strings.TrimSpace(tr.TraceID) == "" {
		return nil
	}

	var start time.Time
	var end time.Time
	hasStart := false
	hasEnd := false
	serviceSeen := map[string]struct{}{}
	hasError := false
	var httpStatus *int32

	for _, span := range tr.Spans {
		if t, err := time.Parse(time.RFC3339Nano, span.StartTime); err == nil {
			if !hasStart || t.Before(start) {
				start = t
				hasStart = true
			}
		}
		if t, err := time.Parse(time.RFC3339Nano, span.EndTime); err == nil {
			if !hasEnd || t.After(end) {
				end = t
				hasEnd = true
			}
		}

		if name := strings.TrimSpace(span.Labels["/component"]); name != "" {
			serviceSeen[name] = struct{}{}
		}
		if name := strings.TrimSpace(span.Labels["/cloud.role"]); name != "" {
			serviceSeen[name] = struct{}{}
		}
		if code := strings.TrimSpace(span.Labels["/http/status_code"]); code != "" {
			if parsed, err := parseInt32(code); err == nil {
				httpStatus = &parsed
				if parsed >= 500 {
					hasError = true
				}
			}
		}
		if strings.TrimSpace(span.Labels["/error/message"]) != "" {
			hasError = true
		}
		if service != "" {
			if role := strings.TrimSpace(span.Labels["/cloud.role"]); role != "" && !strings.Contains(role, service) {
				continue
			}
		}
	}

	dur := 0.0
	if hasStart && hasEnd && end.After(start) {
		dur = end.Sub(start).Seconds()
	}
	svcCount := len(serviceSeen)
	hErr := hasError

	return &TraceSummary{
		ID:           tr.TraceID,
		Duration:     dur,
		ResponseTime: dur,
		HTTPStatus:   httpStatus,
		ServiceCount: &svcCount,
		HasError:     &hErr,
	}
}

func parseInt32(s string) (int32, error) {
	var v int
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return int32(v), nil
	}
	var out int32
	if _, err := fmt.Sscanf(s, "%d", &out); err != nil {
		return 0, err
	}
	return out, nil
}
