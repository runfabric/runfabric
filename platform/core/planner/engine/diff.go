package engine

import "fmt"

func Diff(desired *DesiredState, actual *ActualState, service, stage, provider string) *Plan {
	plan := &Plan{
		Provider: provider,
		Service:  service,
		Stage:    stage,
		Actions:  []PlanAction{},
	}

	actualFunctionMap := map[string]ActualFunction{}
	for _, fn := range actual.Functions {
		actualFunctionMap[fn.Name] = fn
	}

	for _, desiredFunction := range desired.Functions {
		if actualFunction, ok := actualFunctionMap[desiredFunction.Name]; !ok {
			plan.Actions = append(plan.Actions, PlanAction{
				ID:          "function:create:" + desiredFunction.Name,
				Type:        ActionCreate,
				Resource:    ResourceFunction,
				Name:        desiredFunction.Name,
				Description: "Create function",
				Metadata: map[string]string{
					"runtime": desiredFunction.Runtime,
					"handler": desiredFunction.Handler,
				},
			})
		} else {
			changes := map[string][2]string{}

			if actualFunction.Runtime != desiredFunction.Runtime {
				changes["runtime"] = [2]string{actualFunction.Runtime, desiredFunction.Runtime}
			}
			if actualFunction.Handler != desiredFunction.Handler {
				changes["handler"] = [2]string{actualFunction.Handler, desiredFunction.Handler}
			}
			if int(actualFunction.Memory) != desiredFunction.Memory {
				changes["memory"] = [2]string{fmt.Sprintf("%d", actualFunction.Memory), fmt.Sprintf("%d", desiredFunction.Memory)}
			}
			if int(actualFunction.Timeout) != desiredFunction.Timeout {
				changes["timeout"] = [2]string{fmt.Sprintf("%d", actualFunction.Timeout), fmt.Sprintf("%d", desiredFunction.Timeout)}
			}
			if actualFunction.CodeSHA256 != "" && desiredFunction.CodeSHA256 != "" && actualFunction.CodeSHA256 != desiredFunction.CodeSHA256 {
				changes["codeSha256"] = [2]string{actualFunction.CodeSHA256, desiredFunction.CodeSHA256}
			}

			actionType := ActionNoop
			desc := "Function is up to date"
			if len(changes) > 0 {
				actionType = ActionUpdate
				desc = "Update function"
			}

			plan.Actions = append(plan.Actions, PlanAction{
				ID:          "function:reconcile:" + desiredFunction.Name,
				Type:        actionType,
				Resource:    ResourceFunction,
				Name:        desiredFunction.Name,
				Description: desc,
				Changes:     changes,
			})
			delete(actualFunctionMap, desiredFunction.Name)
		}
	}

	for _, stale := range actualFunctionMap {
		plan.Actions = append(plan.Actions, PlanAction{
			ID:          "function:delete:" + stale.Name,
			Type:        ActionDelete,
			Resource:    ResourceFunction,
			Name:        stale.Name,
			Description: "Delete stale function",
		})
	}

	if desired.HTTPAPI != nil && actual.HTTPAPI == nil {
		plan.Actions = append(plan.Actions, PlanAction{
			ID:          "httpapi:create",
			Type:        ActionCreate,
			Resource:    ResourceHTTPAPI,
			Name:        desired.HTTPAPI.Name,
			Description: "Create HTTP API",
		})
	} else if desired.HTTPAPI != nil && actual.HTTPAPI != nil {
		plan.Actions = append(plan.Actions, PlanAction{
			ID:          "httpapi:noop",
			Type:        ActionNoop,
			Resource:    ResourceHTTPAPI,
			Name:        desired.HTTPAPI.Name,
			Description: "HTTP API already exists",
		})
	}

	actualRoutes := map[string]ActualRoute{}
	for _, r := range actual.Routes {
		actualRoutes[r.RouteKey] = r
	}

	for _, dr := range desired.Routes {
		if ar, ok := actualRoutes[dr.RouteKey]; !ok {
			plan.Actions = append(plan.Actions, PlanAction{
				ID:          "route:create:" + dr.RouteKey,
				Type:        ActionCreate,
				Resource:    ResourceHTTPRoute,
				Name:        dr.RouteKey,
				Description: fmt.Sprintf("Create route %s", dr.RouteKey),
				Metadata: map[string]string{
					"function": dr.FunctionName,
				},
			})
		} else {
			plan.Actions = append(plan.Actions, PlanAction{
				ID:          "route:noop:" + dr.RouteKey,
				Type:        ActionNoop,
				Resource:    ResourceHTTPRoute,
				Name:        dr.RouteKey,
				Description: fmt.Sprintf("Route %s already exists", dr.RouteKey),
				Metadata: map[string]string{
					"routeId": ar.ID,
				},
			})
			delete(actualRoutes, dr.RouteKey)
		}
	}

	for _, stale := range actualRoutes {
		plan.Actions = append(plan.Actions, PlanAction{
			ID:          "route:delete:" + stale.RouteKey,
			Type:        ActionDelete,
			Resource:    ResourceHTTPRoute,
			Name:        stale.RouteKey,
			Description: "Delete stale route",
		})
	}

	return plan
}
