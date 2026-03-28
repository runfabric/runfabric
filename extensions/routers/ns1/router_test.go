package ns1

import (
	"context"
	"testing"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

func TestSyncWithClient_DryRunCreate(t *testing.T) {
	client := &stubClient{}
	result, err := syncWithClient(context.Background(), client, "example.com", sdkrouter.RouterSyncRequest{
		DryRun: true,
		Routing: &sdkrouter.RoutingConfig{
			Hostname: "svc.example.com",
			TTL:      60,
			Endpoints: []sdkrouter.RoutingEndpoint{
				{Name: "primary", URL: "https://a.example.com", Weight: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("syncWithClient returned error: %v", err)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action != "create" {
		t.Fatalf("expected create action, got %#v", result.Actions)
	}
	if client.upserted {
		t.Fatal("expected dry-run mode to skip provider writes")
	}
}

func TestSyncWithClient_UpdateAndDeleteCandidate(t *testing.T) {
	client := &stubClient{
		record: &record{
			Zone:   "example.com",
			Domain: "svc.example.com",
			Type:   "CNAME",
			TTL:    60,
			Answers: []answer{
				{Answer: []string{"old.example.com"}, Meta: map[string]any{"weight": 100}},
			},
		},
	}
	result, err := syncWithClient(context.Background(), client, "example.com", sdkrouter.RouterSyncRequest{
		DryRun: false,
		Routing: &sdkrouter.RoutingConfig{
			Hostname: "svc.example.com",
			TTL:      60,
			Endpoints: []sdkrouter.RoutingEndpoint{
				{Name: "primary", URL: "https://new.example.com", Weight: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("syncWithClient returned error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected update + delete-candidate actions, got %#v", result.Actions)
	}
	if result.Actions[0].Action != "delete-candidate" && result.Actions[1].Action != "delete-candidate" {
		t.Fatalf("expected delete-candidate action, got %#v", result.Actions)
	}
	if !client.upserted {
		t.Fatal("expected provider upsert when desired record differs")
	}
}

type stubClient struct {
	record   *record
	upserted bool
}

func (s *stubClient) GetRecord(context.Context, string, string, string) (*record, error) {
	return s.record, nil
}

func (s *stubClient) UpsertRecord(context.Context, string, record) error {
	s.upserted = true
	return nil
}
