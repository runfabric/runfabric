package app

import (
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

func TestRunFabricTargets_Empty(t *testing.T) {
	if got := RunFabricTargets(nil); got != nil {
		t.Fatalf("RunFabricTargets(nil) want nil, got %v", got)
	}
	if got := RunFabricTargets(&config.Config{}); got != nil {
		t.Fatalf("RunFabricTargets(no fabric) want nil, got %v", got)
	}
	if got := RunFabricTargets(&config.Config{Fabric: &config.FabricConfig{}}); got != nil {
		t.Fatalf("RunFabricTargets(empty targets) want nil, got %v", got)
	}
}

func TestRunFabricTargets_ReturnsTargets(t *testing.T) {
	cfg := &config.Config{
		Fabric: &config.FabricConfig{
			Targets: []string{"aws-us", "aws-eu"},
		},
	}
	got := RunFabricTargets(cfg)
	if len(got) != 2 || got[0] != "aws-us" || got[1] != "aws-eu" {
		t.Fatalf("RunFabricTargets want [aws-us aws-eu], got %v", got)
	}
}
