package api

import (
	internalplanner "github.com/runfabric/runfabric/platform/planner/engine"

	engconfig "github.com/runfabric/runfabric/platform/core/model/config"
)

// Plan is a public wrapper over engine/internal/planner.Plan so other
// modules can refer to plans without importing internal packages.
type Plan = internalplanner.Plan

// BuildPlan is a public wrapper over engine/internal/planner.BuildPlan.
func BuildPlan(cfg *engconfig.Config, stage string) *Plan {
	return internalplanner.BuildPlan(cfg, stage)
}
