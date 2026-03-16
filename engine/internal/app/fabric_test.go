package app

import (
	"testing"

	"github.com/runfabric/runfabric/engine/internal/config"
)

func TestFabricTargets_Empty(t *testing.T) {
	if got := FabricTargets(nil); got != nil {
		t.Fatalf("FabricTargets(nil) want nil, got %v", got)
	}
	if got := FabricTargets(&config.Config{}); got != nil {
		t.Fatalf("FabricTargets(no fabric) want nil, got %v", got)
	}
	if got := FabricTargets(&config.Config{Fabric: &config.FabricConfig{}}); got != nil {
		t.Fatalf("FabricTargets(empty targets) want nil, got %v", got)
	}
}

func TestFabricTargets_ReturnsTargets(t *testing.T) {
	cfg := &config.Config{
		Fabric: &config.FabricConfig{
			Targets: []string{"aws-us", "aws-eu"},
		},
	}
	got := FabricTargets(cfg)
	if len(got) != 2 || got[0] != "aws-us" || got[1] != "aws-eu" {
		t.Fatalf("FabricTargets want [aws-us aws-eu], got %v", got)
	}
}
