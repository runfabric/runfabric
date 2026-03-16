package planner

import "fmt"

func Diff(desired *DesiredState, actual *ActualState, service, stage, provider string) *Plan {
	plan := &Plan{
		Provider: provider,
		Service:  service,
		Stage:    stage,
		Actions:  []PlanAction{},
	}

	actualLambdaMap := map[string]ActualLambda{}
	for _, l := range actual.Lambdas {
		actualLambdaMap[l.Name] = l
	}

	for _, dl := range desired.Lambdas {
		if al, ok := actualLambdaMap[dl.Name]; !ok {
			plan.Actions = append(plan.Actions, PlanAction{
				ID:          "lambda:create:" + dl.Name,
				Type:        ActionCreate,
				Resource:    ResourceLambda,
				Name:        dl.Name,
				Description: "Create Lambda function",
				Metadata: map[string]string{
					"runtime": dl.Runtime,
					"handler": dl.Handler,
				},
			})
		} else {
			changes := map[string][2]string{}

			if al.Runtime != dl.Runtime {
				changes["runtime"] = [2]string{al.Runtime, dl.Runtime}
			}
			if al.Handler != dl.Handler {
				changes["handler"] = [2]string{al.Handler, dl.Handler}
			}
			if int(al.Memory) != dl.Memory {
				changes["memory"] = [2]string{fmt.Sprintf("%d", al.Memory), fmt.Sprintf("%d", dl.Memory)}
			}
			if int(al.Timeout) != dl.Timeout {
				changes["timeout"] = [2]string{fmt.Sprintf("%d", al.Timeout), fmt.Sprintf("%d", dl.Timeout)}
			}
			if al.CodeSHA256 != "" && dl.CodeSHA256 != "" && al.CodeSHA256 != dl.CodeSHA256 {
				changes["codeSha256"] = [2]string{al.CodeSHA256, dl.CodeSHA256}
			}

			actionType := ActionNoop
			desc := "Lambda function is up to date"
			if len(changes) > 0 {
				actionType = ActionUpdate
				desc = "Update Lambda function"
			}

			plan.Actions = append(plan.Actions, PlanAction{
				ID:          "lambda:reconcile:" + dl.Name,
				Type:        actionType,
				Resource:    ResourceLambda,
				Name:        dl.Name,
				Description: desc,
				Changes:     changes,
			})
			delete(actualLambdaMap, dl.Name)
		}
	}

	for _, stale := range actualLambdaMap {
		plan.Actions = append(plan.Actions, PlanAction{
			ID:          "lambda:delete:" + stale.Name,
			Type:        ActionDelete,
			Resource:    ResourceLambda,
			Name:        stale.Name,
			Description: "Delete stale Lambda function",
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
