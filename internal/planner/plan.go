package planner

import (
	"fmt"
	"strings"

	"github.com/runfabric/runfabric/internal/config"
)

func BuildPlan(cfg *config.Config, stage string) *Plan {
	plan := &Plan{
		Provider: cfg.Provider.Name,
		Service:  cfg.Service,
		Stage:    stage,
		Actions:  []PlanAction{},
	}

	apiAdded := false

	for fnName, fn := range cfg.Functions {
		buildID := fmt.Sprintf("build:%s", fnName)
		lambdaID := fmt.Sprintf("lambda:%s", fnName)

		plan.Actions = append(plan.Actions, PlanAction{
			ID:          buildID,
			Type:        ActionBuild,
			Resource:    ResourceLambda,
			Name:        fnName,
			Description: fmt.Sprintf("Package function %q with runtime %q", fnName, fn.Runtime),
		})

		plan.Actions = append(plan.Actions, PlanAction{
			ID:          lambdaID,
			Type:        ActionCreate,
			Resource:    ResourceLambda,
			Name:        fnName,
			Description: fmt.Sprintf("Create/update Lambda function for handler %q", fn.Handler),
			DependsOn:   []string{buildID},
		})

		for i, ev := range fn.Events {
			if ev.HTTP != nil {
				if !apiAdded {
					plan.Actions = append(plan.Actions, PlanAction{
						ID:          "httpapi:service",
						Type:        ActionCreate,
						Resource:    ResourceHTTPAPI,
						Name:        cfg.Service,
						Description: fmt.Sprintf("Create/update HTTP API for service %q", cfg.Service),
					})
					apiAdded = true
				}

				httpID := fmt.Sprintf("http:%s:%d", fnName, i)
				plan.Actions = append(plan.Actions, PlanAction{
					ID:          httpID,
					Type:        ActionCreate,
					Resource:    ResourceHTTPAPI,
					Name:        fnName,
					Description: fmt.Sprintf("Create HTTP route %s %s", strings.ToUpper(ev.HTTP.Method), ev.HTTP.Path),
					DependsOn:   []string{"httpapi:service", lambdaID},
				})
			}

			if ev.Cron != "" {
				scheduleID := fmt.Sprintf("schedule:%s:%d", fnName, i)
				plan.Actions = append(plan.Actions, PlanAction{
					ID:          scheduleID,
					Type:        ActionCreate,
					Resource:    ResourceSchedule,
					Name:        fnName,
					Description: fmt.Sprintf("Create cron trigger %q", ev.Cron),
					DependsOn:   []string{lambdaID},
				})
			}
		}
	}

	return plan
}
