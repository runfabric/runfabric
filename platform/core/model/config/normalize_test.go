package config

import "testing"

func TestNormalize_FunctionsArrayWithQueueTrigger(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		FunctionsConfig: []FunctionOverrideConfig{{
			Name:     "worker",
			Entry:    "src/worker.default",
			Triggers: []TriggerRef{{Type: "queue", Queue: "my-queue", BatchSize: 10}},
		}},
	}
	Normalize(cfg)
	if len(cfg.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(cfg.Functions))
	}
	fn := cfg.Functions["worker"]
	if fn.Handler != "src/worker.default" || len(fn.Events) != 1 {
		t.Errorf("unexpected function: %+v", fn)
	}
	if fn.Events[0].Queue == nil || fn.Events[0].Queue.Queue != "my-queue" || fn.Events[0].Queue.Batch != 10 {
		t.Errorf("unexpected queue event: %+v", fn.Events[0])
	}
}

func TestNormalize_AddonNameDefaultsToMapKey(t *testing.T) {
	cfg := &Config{
		Addons: map[string]AddonConfig{
			"sentry": {Version: "1"},
		},
	}
	Normalize(cfg)
	if got := cfg.Addons["sentry"].Name; got != "sentry" {
		t.Fatalf("expected addon name to default to key, got %q", got)
	}
}

func TestNormalize_DeployStrategyDefaultsToAllAtOnce(t *testing.T) {
	cfg := &Config{
		Deploy: &DeployConfig{},
	}
	Normalize(cfg)
	if cfg.Deploy.Strategy != "all-at-once" {
		t.Fatalf("expected deploy.strategy default all-at-once, got %q", cfg.Deploy.Strategy)
	}
}
