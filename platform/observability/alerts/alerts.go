// Package alerts sends webhook and Slack notifications when operations fail.
// Reads alert config from runfabric.yml (alerts.webhook, alerts.slack, alerts.onError, alerts.onTimeout).
package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	coreconfig "github.com/runfabric/runfabric/platform/core/model/config"
)

type alertPayload struct {
	Operation string `json:"operation"`
	Stage     string `json:"stage,omitempty"`
	Service   string `json:"service,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Trigger   string `json:"trigger"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

// NotifyOnError fires configured webhooks/Slack when err is non-nil.
// It is a no-op when configPath is empty, the config cannot be loaded, or no alerts are configured.
func NotifyOnError(configPath, stage, operation string, err error) {
	if err == nil || strings.TrimSpace(configPath) == "" {
		return
	}
	cfg, loadErr := coreconfig.Load(configPath)
	if loadErr != nil {
		return
	}
	cfg, resolveErr := coreconfig.Resolve(cfg, stage)
	if resolveErr != nil {
		return
	}
	if cfg == nil || cfg.Alerts == nil {
		return
	}

	isTimeout := isTimeoutError(err)
	shouldSend := cfg.Alerts.OnError || (isTimeout && cfg.Alerts.OnTimeout)
	if !shouldSend {
		return
	}

	trigger := "error"
	if isTimeout && cfg.Alerts.OnTimeout {
		trigger = "timeout"
	}
	payload := alertPayload{
		Operation: operation,
		Stage:     stage,
		Service:   cfg.Service,
		Provider:  cfg.Provider.Name,
		Trigger:   trigger,
		Error:     err.Error(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	client := &http.Client{Timeout: 5 * time.Second}
	if cfg.Alerts.Webhook != "" {
		postJSON(client, cfg.Alerts.Webhook, payload)
	}
	if cfg.Alerts.Slack != "" {
		postJSON(client, cfg.Alerts.Slack, map[string]any{
			"text": fmt.Sprintf("RunFabric %s %s for service=%s stage=%s provider=%s: %s",
				operation, trigger, cfg.Service, stage, cfg.Provider.Name, err.Error()),
		})
	}
}

// WrapOperation calls fn and fires alert notifications when it returns an error.
func WrapOperation[T any](configPath, stage, operation string, fn func() (T, error)) (T, error) {
	result, err := fn()
	if err != nil {
		NotifyOnError(configPath, stage, operation, err)
	}
	return result, err
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func postJSON(client *http.Client, url string, body any) {
	buf, err := json.Marshal(body)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
