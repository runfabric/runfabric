package route53

import (
	"context"
	"testing"

	route53svc "github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

func TestSyncWithClient_DryRunCreate(t *testing.T) {
	client := &stubClient{}
	result, err := syncWithClient(context.Background(), client, "Z123", sdkrouter.RouterSyncRequest{
		DryRun: true,
		Routing: &sdkrouter.RoutingConfig{
			Hostname: "svc.example.com",
			Strategy: "weighted",
			TTL:      60,
			Endpoints: []sdkrouter.RoutingEndpoint{
				{Name: "a", URL: "https://a.example.com", Weight: 90},
				{Name: "b", URL: "https://b.example.com", Weight: 10},
			},
		},
	})
	if err != nil {
		t.Fatalf("syncWithClient returned error: %v", err)
	}
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 create actions, got %#v", result.Actions)
	}
	if client.changed {
		t.Fatal("expected dry-run not to call ChangeResourceRecordSets")
	}
}

func TestSyncWithClient_NoOp(t *testing.T) {
	routing := &sdkrouter.RoutingConfig{
		Hostname: "svc.example.com",
		Strategy: "weighted",
		TTL:      60,
		Endpoints: []sdkrouter.RoutingEndpoint{
			{Name: "a", URL: "https://a.example.com", Weight: 100},
		},
	}
	records, err := desiredRecords(routing)
	if err != nil {
		t.Fatalf("desiredRecords returned error: %v", err)
	}
	client := &stubClient{existing: records}

	result, err := syncWithClient(context.Background(), client, "Z123", sdkrouter.RouterSyncRequest{
		DryRun:  false,
		Routing: routing,
	})
	if err != nil {
		t.Fatalf("syncWithClient returned error: %v", err)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action != "no-op" {
		t.Fatalf("expected no-op action, got %#v", result.Actions)
	}
	if client.changed {
		t.Fatal("expected no provider change call when record is already aligned")
	}
}

type stubClient struct {
	existing []route53types.ResourceRecordSet
	changed  bool
}

func (s *stubClient) ListResourceRecordSets(context.Context, *route53svc.ListResourceRecordSetsInput, ...func(*route53svc.Options)) (*route53svc.ListResourceRecordSetsOutput, error) {
	return &route53svc.ListResourceRecordSetsOutput{
		ResourceRecordSets: s.existing,
	}, nil
}

func (s *stubClient) ChangeResourceRecordSets(context.Context, *route53svc.ChangeResourceRecordSetsInput, ...func(*route53svc.Options)) (*route53svc.ChangeResourceRecordSetsOutput, error) {
	s.changed = true
	return &route53svc.ChangeResourceRecordSetsOutput{}, nil
}
