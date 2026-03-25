package cloudflare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/deploy/apiutil"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func TestLoggerLogs_UsesWranglerTailWhenAvailable(t *testing.T) {
	oldTail := wranglerTailProvider
	wranglerTailProvider = func(ctx context.Context, workerName string) ([]string, error) {
		if workerName != "demo-dev" {
			t.Fatalf("unexpected worker name: %s", workerName)
		}
		return []string{"{\"event\":\"tail\"}"}, nil
	}
	defer func() { wranglerTailProvider = oldTail }()

	logger := Logger{}
	cfg := sdkprovider.Config{"service": "demo"}
	result, err := logger.Logs(context.Background(), cfg, "dev", "api", &state.Receipt{Metadata: map[string]string{"worker": "demo-dev"}})
	if err != nil {
		t.Fatalf("logs failed: %v", err)
	}
	if result == nil || len(result.Lines) != 1 {
		t.Fatalf("expected wrangler lines, got %#v", result)
	}
	if !strings.Contains(result.Lines[0], "tail") {
		t.Fatalf("unexpected line: %q", result.Lines[0])
	}
}

func TestLoggerLogs_FallsBackToAPITailWhenWranglerUnavailable(t *testing.T) {
	oldTail := wranglerTailProvider
	wranglerTailProvider = func(ctx context.Context, workerName string) ([]string, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() { wranglerTailProvider = oldTail }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("line-1\nline-2\n"))
	}))
	defer ts.Close()

	oldCFAPI := cfAPI
	cfAPI = ts.URL
	defer func() { cfAPI = oldCFAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "acct")
	t.Setenv("CLOUDFLARE_API_TOKEN", "token")

	logger := Logger{}
	cfg := sdkprovider.Config{"service": "demo"}
	result, err := logger.Logs(context.Background(), cfg, "dev", "api", &state.Receipt{Metadata: map[string]string{"worker": "demo-dev"}})
	if err != nil {
		t.Fatalf("logs failed: %v", err)
	}
	if result == nil || len(result.Lines) != 2 {
		t.Fatalf("expected API tail lines, got %#v", result)
	}
	if result.Lines[0] != "line-1" {
		t.Fatalf("unexpected first line: %q", result.Lines[0])
	}
}

func TestLoggerLogs_DisableWranglerTailEnv(t *testing.T) {
	oldTail := wranglerTailProvider
	wranglerTailProvider = func(ctx context.Context, workerName string) ([]string, error) {
		return []string{"should-not-be-used"}, nil
	}
	defer func() { wranglerTailProvider = oldTail }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("api-line"))
	}))
	defer ts.Close()

	oldCFAPI := cfAPI
	cfAPI = ts.URL
	defer func() { cfAPI = oldCFAPI }()

	oldClient := apiutil.DefaultClient
	apiutil.DefaultClient = ts.Client()
	defer func() { apiutil.DefaultClient = oldClient }()

	t.Setenv("RUNFABRIC_CLOUDFLARE_DISABLE_WRANGLER_TAIL", "1")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "acct")
	t.Setenv("CLOUDFLARE_API_TOKEN", "token")

	logger := Logger{}
	cfg := sdkprovider.Config{"service": "demo"}
	result, err := logger.Logs(context.Background(), cfg, "dev", "api", &state.Receipt{Metadata: map[string]string{"worker": "demo-dev"}})
	if err != nil {
		t.Fatalf("logs failed: %v", err)
	}
	if result == nil || len(result.Lines) != 1 || result.Lines[0] != "api-line" {
		t.Fatalf("expected API fallback line, got %#v", result)
	}
}

var _ providers.LogsResult
