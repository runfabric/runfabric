package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Logger fetches Worker logs (tail API or dashboard link).
type Logger struct{}

var wranglerTailProvider = fetchWranglerTailLines

func (Logger) Logs(ctx context.Context, cfg sdkprovider.Config, stage, function string, receipt any) (*sdkprovider.LogsResult, error) {
	service := sdkprovider.Service(cfg)
	rv := sdkprovider.DecodeReceipt(receipt)
	accountID := sdkprovider.Env("CLOUDFLARE_ACCOUNT_ID")
	workerName := rv.Metadata["worker"]
	if workerName == "" {
		workerName = fmt.Sprintf("%s-%s", service, stage)
	}
	if strings.TrimSpace(os.Getenv("RUNFABRIC_CLOUDFLARE_DISABLE_WRANGLER_TAIL")) != "1" {
		if lines, err := wranglerTailProvider(ctx, workerName); err == nil && len(lines) > 0 {
			return &sdkprovider.LogsResult{Provider: "cloudflare-workers", Function: function, Lines: lines}, nil
		}
	}
	url := cfAPI + "/accounts/" + accountID + "/workers/scripts/" + workerName + "/tail"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+sdkprovider.Env("CLOUDFLARE_API_TOKEN"))
	resp, err := sdkprovider.DefaultClient.Do(req)
	if err != nil {
		return &sdkprovider.LogsResult{
			Provider: "cloudflare-workers",
			Function: function,
			Lines:    []string{fmt.Sprintf("Logs: %v. Dashboard: https://dash.cloudflare.com/ (Workers)", err)},
		}, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return &sdkprovider.LogsResult{
			Provider: "cloudflare-workers",
			Function: function,
			Lines:    []string{fmt.Sprintf("Logs API: %s. Dashboard: https://dash.cloudflare.com/ (Workers)", string(b))},
		}, nil
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{"No tail output. Use dashboard for Real-time Logs."}
	}
	return &sdkprovider.LogsResult{Provider: "cloudflare-workers", Function: function, Lines: lines}, nil
}

func fetchWranglerTailLines(ctx context.Context, workerName string) ([]string, error) {
	if _, err := exec.LookPath("wrangler"); err != nil {
		return nil, err
	}
	// wrangler tail is a stream; collect a short sample and then stop.
	tailCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tailCtx, "wrangler", "tail", workerName, "--format", "json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	raw := strings.TrimSpace(stdout.String())
	if raw == "" {
		raw = strings.TrimSpace(stderr.String())
	}
	if raw == "" {
		return nil, err
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
		if len(lines) >= 80 {
			break
		}
	}
	if len(lines) == 0 {
		return nil, err
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(tailCtx.Err(), context.DeadlineExceeded) {
			return lines, nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "killed") {
			return lines, nil
		}
	}
	return lines, nil
}
