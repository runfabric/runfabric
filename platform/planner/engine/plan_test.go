package engine

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestBuildPlan_EmptyFunctions(t *testing.T) {
	cfg := &config.Config{
		Service:   "svc",
		Provider:  config.ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]config.FunctionConfig{},
	}
	plan := BuildPlan(cfg, "dev")
	if plan.Provider != "aws-lambda" || plan.Service != "svc" || plan.Stage != "dev" {
		t.Errorf("plan metadata: Provider=%q Service=%q Stage=%q", plan.Provider, plan.Service, plan.Stage)
	}
	if len(plan.Actions) != 0 {
		t.Errorf("expected 0 actions for no functions, got %d", len(plan.Actions))
	}
}

func TestBuildPlan_OneFunctionNoEvents(t *testing.T) {
	cfg := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]config.FunctionConfig{
			"api": {Handler: "index.handler"},
		},
	}
	plan := BuildPlan(cfg, "dev")
	// build:api + function:api
	if len(plan.Actions) != 2 {
		t.Fatalf("expected 2 actions (build+function), got %d", len(plan.Actions))
	}
	if plan.Actions[0].ID != "build:api" || plan.Actions[0].Type != ActionBuild {
		t.Errorf("first action: id=%q type=%q", plan.Actions[0].ID, plan.Actions[0].Type)
	}
	if plan.Actions[1].ID != "function:api" || len(plan.Actions[1].DependsOn) != 1 || plan.Actions[1].DependsOn[0] != "build:api" {
		t.Errorf("second action: id=%q DependsOn=%v", plan.Actions[1].ID, plan.Actions[1].DependsOn)
	}
}

func TestBuildPlan_OneFunctionHTTPAndCron(t *testing.T) {
	cfg := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]config.FunctionConfig{
			"api": {
				Handler: "index.handler",
				Events: []config.EventConfig{
					{HTTP: &config.HTTPEvent{Path: "/", Method: "GET"}},
					{Cron: "0 * * * *"},
				},
			},
		},
	}
	plan := BuildPlan(cfg, "dev")
	// build:api, function:api, httpapi:service, http:api:0, schedule:api:1
	if len(plan.Actions) != 5 {
		t.Fatalf("expected 5 actions, got %d: %+v", len(plan.Actions), plan.Actions)
	}
	var hasHTTPAPI, hasHTTPRoute, hasSchedule bool
	for _, a := range plan.Actions {
		if a.ID == "httpapi:service" {
			hasHTTPAPI = true
		}
		if a.ID == "http:api:0" && a.Resource == ResourceHTTPAPI {
			hasHTTPRoute = true
		}
		if a.ID == "schedule:api:1" && a.Resource == ResourceSchedule {
			hasSchedule = true
		}
	}
	if !hasHTTPAPI || !hasHTTPRoute || !hasSchedule {
		t.Errorf("missing expected actions: httpapi=%v httpRoute=%v schedule=%v", hasHTTPAPI, hasHTTPRoute, hasSchedule)
	}
}

func TestBuildPlan_TwoFunctions(t *testing.T) {
	cfg := &config.Config{
		Service:  "svc",
		Provider: config.ProviderConfig{Name: "aws-lambda", Runtime: "nodejs"},
		Functions: map[string]config.FunctionConfig{
			"api":    {Handler: "api.handler"},
			"worker": {Handler: "worker.handler"},
		},
	}
	plan := BuildPlan(cfg, "dev")
	// build:api, function:api, build:worker, function:worker = 4
	if len(plan.Actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(plan.Actions))
	}
	ids := make(map[string]bool)
	for _, a := range plan.Actions {
		ids[a.ID] = true
	}
	for _, id := range []string{"build:api", "function:api", "build:worker", "function:worker"} {
		if !ids[id] {
			t.Errorf("missing action %q", id)
		}
	}
}
