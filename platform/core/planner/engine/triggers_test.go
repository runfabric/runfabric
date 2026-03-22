package engine

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestTriggerKindFromEvent(t *testing.T) {
	if got := TriggerKindFromEvent(config.EventConfig{HTTP: &config.HTTPEvent{}}); got != TriggerHTTP {
		t.Errorf("got %q", got)
	}
	if got := TriggerKindFromEvent(config.EventConfig{Cron: "0 0 * * *"}); got != TriggerCron {
		t.Errorf("got %q", got)
	}
	if got := TriggerKindFromEvent(config.EventConfig{Queue: &config.QueueEvent{Queue: "my-queue"}}); got != TriggerQueue {
		t.Errorf("got %q", got)
	}
	if got := TriggerKindFromEvent(config.EventConfig{Storage: &config.StorageEvent{Bucket: "b"}}); got != TriggerStorage {
		t.Errorf("got %q", got)
	}
	if got := TriggerKindFromEvent(config.EventConfig{EventBridge: &config.EventBridgeEvent{}}); got != TriggerEventBridge {
		t.Errorf("got %q", got)
	}
	if got := TriggerKindFromEvent(config.EventConfig{PubSub: &config.PubSubEvent{Topic: "t"}}); got != TriggerPubSub {
		t.Errorf("got %q", got)
	}
	if got := TriggerKindFromEvent(config.EventConfig{}); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestExtractTriggers(t *testing.T) {
	cfg := &config.Config{
		Service: "svc",
		Functions: map[string]config.FunctionConfig{
			"fn1": {
				Handler: "h",
				Events: []config.EventConfig{
					{HTTP: &config.HTTPEvent{Path: "/", Method: "GET"}},
					{Cron: "0 * * * *"},
				},
			},
		},
	}
	out := ExtractTriggers(cfg)
	if len(out) != 1 || out[0].Function != "fn1" || len(out[0].Specs) != 2 {
		t.Errorf("ExtractTriggers: got %+v", out)
	}
}

func TestValidateTriggersForProvider(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{Name: "fly-machines"},
		Functions: map[string]config.FunctionConfig{
			"fn1": {Handler: "h", Events: []config.EventConfig{{Cron: "0 0 * * *"}}},
		},
	}
	errs := ValidateTriggersForProvider(cfg, "fly-machines")
	if len(errs) == 0 {
		t.Error("expected error: fly-machines does not support cron")
	}
	errs = ValidateTriggersForProvider(cfg, "aws-lambda")
	if len(errs) != 0 {
		t.Errorf("aws-lambda supports cron, got %v", errs)
	}
}
