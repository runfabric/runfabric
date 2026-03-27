package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type capturedAlertRequest struct {
	Path string
	Body map[string]any
}

func TestNotifyAlertsForError_SendsWebhookAndSlackOnError(t *testing.T) {
	var mu sync.Mutex
	var requests []capturedAlertRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode alert body: %v", err)
		}
		mu.Lock()
		requests = append(requests, capturedAlertRequest{Path: r.URL.Path, Body: body})
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfgPath := writeAlertsConfig(t, t.TempDir(), server.URL+"/webhook", server.URL+"/slack", true, false)
	notifyAlertsForError(cfgPath, "dev", "plan", errors.New("boom"))

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("expected 2 alert requests, got %d", len(requests))
	}
	paths := map[string]map[string]any{}
	for _, req := range requests {
		paths[req.Path] = req.Body
	}
	if body := paths["/webhook"]; body["operation"] != "plan" || body["trigger"] != "error" {
		t.Fatalf("unexpected webhook body: %#v", body)
	}
	if body := paths["/slack"]; body["text"] == nil {
		t.Fatalf("expected slack text body, got %#v", body)
	}
}

func TestNotifyAlertsForError_TimeoutHonorsOnTimeout(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfgPath := writeAlertsConfig(t, t.TempDir(), server.URL+"/webhook", "", false, true)
	notifyAlertsForError(cfgPath, "dev", "invoke", context.DeadlineExceeded)
	if count != 1 {
		t.Fatalf("expected timeout alert to be sent once, got %d", count)
	}
}

func TestNotifyAlertsForError_DisabledDoesNothing(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfgPath := writeAlertsConfig(t, t.TempDir(), server.URL+"/webhook", "", false, false)
	notifyAlertsForError(cfgPath, "dev", "remove", errors.New("boom"))
	if count != 0 {
		t.Fatalf("expected no alerts when disabled, got %d", count)
	}
}

func writeAlertsConfig(t *testing.T, dir, webhookURL, slackURL string, onError, onTimeout bool) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "runfabric.yml")
	content := "service: test-service\nprovider:\n  name: aws-lambda\n  runtime: nodejs20.x\n  region: us-east-1\nalerts:\n"
	if webhookURL != "" {
		content += "  webhook: \"" + webhookURL + "\"\n"
	}
	if slackURL != "" {
		content += "  slack: \"" + slackURL + "\"\n"
	}
	if onError {
		content += "  onError: true\n"
	}
	if onTimeout {
		content += "  onTimeout: true\n"
	}
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write runfabric.yml: %v", err)
	}
	return cfgPath
}
