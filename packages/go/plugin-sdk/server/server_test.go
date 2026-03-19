package server

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestServerHandshakeAndMethodDispatch(t *testing.T) {
	s := New(Options{
		ProtocolVersion: "2025-01-01",
		Methods: map[string]MethodFunc{
			"provider.doctor": func(ctx context.Context, params json.RawMessage) (any, error) {
				return map[string]any{"checks": []string{"ok"}}, nil
			},
		},
	})

	in := bytes.NewBufferString(
		`{"id":"1","method":"handshake"}` + "\n" +
			`{"id":"2","method":"provider.doctor","params":{"stage":"dev"}}` + "\n",
	)
	var out bytes.Buffer
	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("serve: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}

	var first map[string]any
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if first["id"] != "1" {
		t.Fatalf("first response id=%v want 1", first["id"])
	}
	result1, ok := first["result"].(map[string]any)
	if !ok || result1["protocolVersion"] != "2025-01-01" {
		t.Fatalf("unexpected handshake result: %#v", first["result"])
	}

	var second map[string]any
	if err := json.Unmarshal(lines[1], &second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if second["id"] != "2" {
		t.Fatalf("second response id=%v want 2", second["id"])
	}
}
