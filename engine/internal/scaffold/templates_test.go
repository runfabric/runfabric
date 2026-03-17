package scaffold

import (
	"reflect"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/engine/internal/planner"
)

func TestHandlerContent(t *testing.T) {
	tests := []struct {
		lang    string
		trigger string
		wantExt string
		wantSub string
	}{
		{"js", planner.TriggerHTTP, ".js", "exports.handler"},
		{"ts", planner.TriggerHTTP, ".ts", "event: any"},
		{"node", planner.TriggerCron, ".js", "Cron triggered"},
		{"python", planner.TriggerHTTP, ".py", "def handler"},
		{"python", planner.TriggerQueue, ".py", "def handler"},
		{"go", planner.TriggerHTTP, ".go", "package main"},
		{"go", planner.TriggerCron, ".go", "package main"},
	}
	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.trigger, func(t *testing.T) {
			got, ok := HandlerContent(tt.lang, tt.trigger)
			if !ok {
				t.Fatal("HandlerContent should return ok true")
			}
			if got.Ext != tt.wantExt {
				t.Errorf("Ext = %q, want %q", got.Ext, tt.wantExt)
			}
			if !strings.Contains(got.Content, tt.wantSub) {
				t.Errorf("Content should contain %q, got: %s", tt.wantSub, got.Content)
			}
		})
	}
}

func TestHandlerContent_jsAlias(t *testing.T) {
	// js normalizes to node and produces .js
	got, ok := HandlerContent("js", planner.TriggerHTTP)
	if !ok {
		t.Fatal("HandlerContent(js) should return ok")
	}
	if got.Ext != ".js" {
		t.Errorf("js should produce .js, got %q", got.Ext)
	}
	if !strings.Contains(got.Content, "exports.handler") {
		t.Errorf("js should produce Node handler, got: %s", got.Content)
	}
}

func TestBuildFunctionEntry_HTTP(t *testing.T) {
	entry := BuildFunctionEntry("src/api.handler", planner.TriggerHTTP, "GET:/hello", "", "")
	if entry["handler"] != "src/api.handler" {
		t.Errorf("handler = %v", entry["handler"])
	}
	if entry["memory"] != 128 {
		t.Errorf("memory = %v", entry["memory"])
	}
	events, _ := entry["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("events length = %d", len(events))
	}
	ev, _ := events[0].(map[string]any)
	http, _ := ev["http"].(map[string]any)
	if http["method"] != "get" {
		t.Errorf("http.method = %v", http["method"])
	}
	if http["path"] != "/hello" {
		t.Errorf("http.path = %v", http["path"])
	}
}

func TestBuildFunctionEntry_Cron(t *testing.T) {
	entry := BuildFunctionEntry("src/cron.handler", planner.TriggerCron, "", "rate(1 hour)", "")
	events, _ := entry["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("events length = %d", len(events))
	}
	ev, _ := events[0].(map[string]any)
	if ev["cron"] != "rate(1 hour)" {
		t.Errorf("cron = %v", ev["cron"])
	}
}

func TestBuildFunctionEntry_CronDefault(t *testing.T) {
	entry := BuildFunctionEntry("src/cron.handler", planner.TriggerCron, "", "", "")
	events, _ := entry["events"].([]any)
	ev, _ := events[0].(map[string]any)
	if ev["cron"] != "rate(5 minutes)" {
		t.Errorf("default cron = %v", ev["cron"])
	}
}

func TestBuildFunctionEntry_Queue(t *testing.T) {
	entry := BuildFunctionEntry("src/worker.handler", planner.TriggerQueue, "", "", "my-queue")
	events, _ := entry["events"].([]any)
	ev, _ := events[0].(map[string]any)
	queue, _ := ev["queue"].(map[string]any)
	if queue["queue"] != "my-queue" {
		t.Errorf("queue.queue = %v", queue["queue"])
	}
}

func TestBuildFunctionEntry_QueueDefault(t *testing.T) {
	entry := BuildFunctionEntry("src/worker.handler", planner.TriggerQueue, "", "", "")
	events, _ := entry["events"].([]any)
	ev, _ := events[0].(map[string]any)
	queue, _ := ev["queue"].(map[string]any)
	if queue["queue"] != "my-queue" {
		t.Errorf("default queue = %v", queue["queue"])
	}
}

func TestBuildFunctionEntry_HTTPRouteParse(t *testing.T) {
	entry := BuildFunctionEntry("src/handler", planner.TriggerHTTP, "POST:/api/items", "", "")
	events, _ := entry["events"].([]any)
	ev, _ := events[0].(map[string]any)
	http, _ := ev["http"].(map[string]any)
	if http["method"] != "post" {
		t.Errorf("method = %v", http["method"])
	}
	if http["path"] != "/api/items" {
		t.Errorf("path = %v", http["path"])
	}
}

func TestBuildFunctionEntry_Structure(t *testing.T) {
	entry := BuildFunctionEntry("x/handler", planner.TriggerHTTP, "GET:/", "", "")
	if _, ok := entry["handler"]; !ok {
		t.Error("entry should have handler")
	}
	if _, ok := entry["memory"]; !ok {
		t.Error("entry should have memory")
	}
	if _, ok := entry["timeout"]; !ok {
		t.Error("entry should have timeout")
	}
	events, ok := entry["events"].([]any)
	if !ok || len(events) == 0 {
		t.Error("entry should have non-empty events")
	}
	// Should be serializable (map[string]any style)
	_ = reflect.ValueOf(entry).Kind()
}

func TestBuildResourceEntry(t *testing.T) {
	entry := BuildResourceEntry("database", "DATABASE_URL")
	if entry["type"] != "database" {
		t.Errorf("type = %v", entry["type"])
	}
	if entry["connectionStringEnv"] != "DATABASE_URL" || entry["envVar"] != "DATABASE_URL" {
		t.Errorf("connection env not set: %v", entry)
	}
	entry2 := BuildResourceEntry("cache", "REDIS_URL")
	if entry2["type"] != "cache" {
		t.Errorf("type = %v", entry2["type"])
	}
}

func TestBuildAddonEntry(t *testing.T) {
	entry := BuildAddonEntry("1.0.0")
	if entry["version"] != "1.0.0" {
		t.Errorf("version = %v", entry["version"])
	}
	empty := BuildAddonEntry("")
	if len(empty) != 0 {
		t.Errorf("empty version should give empty map: %v", empty)
	}
}

func TestBuildProviderOverrideEntry(t *testing.T) {
	entry := BuildProviderOverrideEntry("aws-lambda", "nodejs20.x", "us-east-1")
	if entry["name"] != "aws-lambda" {
		t.Errorf("name = %v", entry["name"])
	}
	if entry["runtime"] != "nodejs20.x" || entry["region"] != "us-east-1" {
		t.Errorf("runtime/region = %v", entry)
	}
}
