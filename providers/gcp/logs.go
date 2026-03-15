package gcp

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/internal/apiutil"
	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"
	"github.com/runfabric/runfabric/internal/state"
)

// Logger returns a link to Cloud Console (Cloud Logging requires project/scoping).
type Logger struct{}

func (Logger) Logs(ctx context.Context, cfg *config.Config, stage, function string, receipt *state.Receipt) (*providers.LogsResult, error) {
	project := apiutil.Env("GCP_PROJECT")
	region := receipt.Outputs["region"]
	if region == "" {
		region = "us-central1"
	}
	line := fmt.Sprintf("View logs: https://console.cloud.google.com/functions/list?project=%s (region %s)", project, region)
	if project == "" {
		line = "Set GCP_PROJECT; see Cloud Functions console for logs."
	}
	return &providers.LogsResult{Provider: "gcp-functions", Function: function, Lines: []string{line}}, nil
}
