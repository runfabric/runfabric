package integration

import (
	"os"
	"testing"

	app "github.com/runfabric/runfabric/platform/workflow/app"
)

func TestPrepareDevStreamTunnelAWSIfEnabled(t *testing.T) {
	if os.Getenv("RUNFABRIC_AWS_INTEGRATION") != "1" {
		t.Skip("set RUNFABRIC_AWS_INTEGRATION=1 to enable real AWS dev-stream integration test")
	}
	configPath := os.Getenv("RUNFABRIC_AWS_DEV_STREAM_CONFIG")
	stage := os.Getenv("RUNFABRIC_AWS_DEV_STREAM_STAGE")
	tunnelURL := os.Getenv("RUNFABRIC_AWS_DEV_STREAM_TUNNEL_URL")
	if configPath == "" || stage == "" || tunnelURL == "" {
		t.Skip("set RUNFABRIC_AWS_DEV_STREAM_CONFIG, RUNFABRIC_AWS_DEV_STREAM_STAGE, and RUNFABRIC_AWS_DEV_STREAM_TUNNEL_URL to run dev-stream integration test")
	}
	restore, report, err := app.PrepareDevStreamTunnelWithReport(configPath, stage, tunnelURL)
	if err != nil {
		t.Fatalf("PrepareDevStreamTunnelWithReport failed: %v", err)
	}
	if restore == nil {
		t.Fatal("expected restore function from AWS dev-stream prepare")
	}
	if report == nil {
		t.Fatal("expected dev-stream report")
	}
	if report.EffectiveMode != "route-rewrite" {
		t.Fatalf("expected route-rewrite mode, got %+v", report)
	}
	restore()
}
