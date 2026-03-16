package config

import (
	"testing"
)

func TestNormalize_ReferenceFormatProvider(t *testing.T) {
	cfg := &Config{
		Providers: []string{"aws-lambda"},
		Runtime:   "nodejs",
	}
	Normalize(cfg)
	if cfg.Provider.Name != "aws-lambda" || cfg.Provider.Runtime != "nodejs" {
		t.Errorf("expected provider aws-lambda/nodejs, got %+v", cfg.Provider)
	}
}

func TestNormalize_StateToBackend(t *testing.T) {
	cfg := &Config{
		State: &StateConfig{
			Backend: "s3",
			S3:      &StateS3{Bucket: "my-bucket", KeyPrefix: "state/"},
		},
	}
	Normalize(cfg)
	if cfg.Backend == nil {
		t.Fatal("expected backend from state")
	}
	if cfg.Backend.Kind != "s3" || cfg.Backend.S3Bucket != "my-bucket" || cfg.Backend.S3Prefix != "state/" {
		t.Errorf("unexpected backend: %+v", cfg.Backend)
	}
}

func TestNormalize_EntryAndTriggersCreatesApiFunction(t *testing.T) {
	cfg := &Config{
		Runtime:  "nodejs",
		Entry:    "src/index.ts",
		Triggers: []TriggerRef{{Type: "http", Method: "GET", Path: "/hello"}},
	}
	Normalize(cfg)
	if len(cfg.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(cfg.Functions))
	}
	fn, ok := cfg.Functions["api"]
	if !ok {
		t.Fatal("expected function api")
	}
	if fn.Handler != "src/index.ts" || len(fn.Events) != 1 {
		t.Errorf("unexpected function: %+v", fn)
	}
	if fn.Events[0].HTTP == nil || fn.Events[0].HTTP.Path != "/hello" {
		t.Errorf("unexpected event: %+v", fn.Events[0])
	}
}

func TestNormalize_FunctionsArrayWithQueueTrigger(t *testing.T) {
	cfg := &Config{
		Provider: ProviderConfig{Name: "aws", Runtime: "nodejs"},
		FunctionsData: &FunctionsRaw{
			AsArray: []FunctionOverrideConfig{{
				Name:     "worker",
				Entry:    "src/worker.default",
				Triggers: []TriggerRef{{Type: "queue", Queue: "my-queue", BatchSize: 10}},
			}},
		},
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
