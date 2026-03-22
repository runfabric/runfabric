package unit

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	observability "github.com/runfabric/runfabric/platform/observability/core"
)

func TestEmitEventOutputsValidJSON(t *testing.T) {
	out := captureStdout(t, func() {
		err := observability.Emit(&observability.Event{
			Type:      "deploy-start",
			Service:   "svc",
			Stage:     "dev",
			Message:   "deploy started",
			Timestamp: "2026-03-15T00:00:00Z",
			Metadata: map[string]string{
				"attempt": "1",
			},
		})
		if err != nil {
			t.Fatalf("Emit failed: %v", err)
		}
	})

	var evt observability.Event
	if err := json.Unmarshal(bytes.TrimSpace(out), &evt); err != nil {
		t.Fatalf("expected valid JSON event, got error: %v, payload=%s", err, string(out))
	}

	if evt.Type != "deploy-start" {
		t.Fatalf("expected type=deploy-start, got %q", evt.Type)
	}
}

func TestEmitEventIncludesRequiredFields(t *testing.T) {
	out := captureStdout(t, func() {
		err := observability.Emit(&observability.Event{
			Type:      "deploy-complete",
			Service:   "svc",
			Stage:     "dev",
			Timestamp: "2026-03-15T00:00:00Z",
		})
		if err != nil {
			t.Fatalf("Emit failed: %v", err)
		}
	})

	var evt map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out), &evt); err != nil {
		t.Fatalf("unmarshal event failed: %v", err)
	}

	if evt["type"] != "deploy-complete" {
		t.Fatalf("expected type=deploy-complete, got %#v", evt["type"])
	}
	if evt["service"] != "svc" {
		t.Fatalf("expected service=svc, got %#v", evt["service"])
	}
	if evt["stage"] != "dev" {
		t.Fatalf("expected stage=dev, got %#v", evt["stage"])
	}
	if evt["timestamp"] == "" {
		t.Fatal("expected non-empty timestamp")
	}
}

func TestEmitEventSerializesMetadata(t *testing.T) {
	out := captureStdout(t, func() {
		err := observability.Emit(&observability.Event{
			Type:      "retry",
			Service:   "svc",
			Stage:     "dev",
			Timestamp: "2026-03-15T00:00:00Z",
			Metadata: map[string]string{
				"attempt": "2",
				"reason":  "conflict",
			},
		})
		if err != nil {
			t.Fatalf("Emit failed: %v", err)
		}
	})

	var evt map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out), &evt); err != nil {
		t.Fatalf("unmarshal event failed: %v", err)
	}

	md, ok := evt["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata object, got %#v", evt["metadata"])
	}
	if md["attempt"] != "2" {
		t.Fatalf("expected metadata.attempt=2, got %#v", md["attempt"])
	}
	if md["reason"] != "conflict" {
		t.Fatalf("expected metadata.reason=conflict, got %#v", md["reason"])
	}
}

func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w

	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.Bytes()
	}()

	fn()

	_ = w.Close()
	os.Stdout = old

	return <-done
}
