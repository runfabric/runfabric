package azuretrafficmanager

import (
	"context"
	"testing"

	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

func TestSyncWithClient_DryRunCreate(t *testing.T) {
	client := &stubClient{
		profile: &profile{},
	}
	result, err := syncWithClient(context.Background(), client, "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.Network/trafficManagerProfiles/p1", sdkrouter.RouterSyncRequest{
		DryRun: true,
		Routing: &sdkrouter.RoutingConfig{
			Hostname: "svc.example.com",
			Strategy: "weighted",
			Endpoints: []sdkrouter.RoutingEndpoint{
				{Name: "a", URL: "https://a.example.com", Weight: 70},
				{Name: "b", URL: "https://b.example.com", Weight: 30},
			},
		},
	})
	if err != nil {
		t.Fatalf("syncWithClient returned error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 create actions, got %#v", result.Actions)
	}
	if client.upserts != 0 {
		t.Fatal("expected no provider writes in dry-run")
	}
}

func TestSyncWithClient_UpdateAndDeleteCandidate(t *testing.T) {
	client := &stubClient{
		profile: &profile{
			Properties: profileProperties{
				Endpoints: []endpointResource{
					{
						Name: "runfabric-a",
						Properties: endpointProperties{
							Target:         "old.example.com",
							EndpointStatus: "Enabled",
							Weight:         100,
						},
					},
					{
						Name: "runfabric-stale",
						Properties: endpointProperties{
							Target:         "stale.example.com",
							EndpointStatus: "Enabled",
							Weight:         1,
						},
					},
				},
			},
		},
	}
	result, err := syncWithClient(context.Background(), client, "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.Network/trafficManagerProfiles/p1", sdkrouter.RouterSyncRequest{
		DryRun: false,
		Routing: &sdkrouter.RoutingConfig{
			Hostname: "svc.example.com",
			Strategy: "weighted",
			Endpoints: []sdkrouter.RoutingEndpoint{
				{Name: "a", URL: "https://new.example.com", Weight: 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("syncWithClient returned error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected update + delete-candidate actions, got %#v", result.Actions)
	}
	if client.upserts != 1 {
		t.Fatalf("expected one provider upsert, got %d", client.upserts)
	}
}

type stubClient struct {
	profile *profile
	upserts int
}

func (s *stubClient) GetProfile(context.Context, string) (*profile, error) {
	if s.profile == nil {
		return &profile{}, nil
	}
	return s.profile, nil
}

func (s *stubClient) UpsertEndpoint(context.Context, string, endpointResource) error {
	s.upserts++
	return nil
}
