package app

import (
	"fmt"

	state "github.com/runfabric/runfabric/platform/core/state/core"
	workflow "github.com/runfabric/runfabric/platform/workflow/core"
)

// ComposeDeployResult is the result of compose deploy (one entry per service).
type ComposeDeployResult struct {
	Service string            `json:"service"`
	Result  any               `json:"result,omitempty"`
	Error   string            `json:"error,omitempty"`
	Outputs map[string]string `json:"outputs,omitempty"`
}

// ComposeDeployResultList is the full result of runfabric compose deploy.
type ComposeDeployResultList struct {
	Services []ComposeDeployResult `json:"services"`
}

// ServiceURLFromReceipt returns the primary URL for a deployed service from its receipt outputs.
// Tries "ServiceURL", "url", "ApiUrl" in that order.
func ServiceURLFromReceipt(outputs map[string]string) string {
	if outputs == nil {
		return ""
	}
	for _, k := range []string{"ServiceURL", "url", "ApiUrl"} {
		if v, ok := outputs[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// ComposeDeploy loads the compose file, deploys each service in dependency order, and injects
// SERVICE_*_URL from prior services' receipt outputs into dependent services.
func ComposeDeploy(composePath, stage string, rollbackOnFailure, noRollbackOnFailure bool) (any, error) {
	c, err := workflow.LoadCompose(composePath)
	if err != nil {
		return nil, err
	}
	configPaths, err := workflow.ResolveServiceConfigPaths(composePath, c)
	if err != nil {
		return nil, err
	}
	order, err := workflow.TopoOrder(c)
	if err != nil {
		return nil, err
	}

	serviceURLs := make(map[string]string)
	var results []ComposeDeployResult

	for _, name := range order {
		configPath := configPaths[name]
		bindingEnv := workflow.ServiceBindingEnv(serviceURLs)

		result, err := Deploy(configPath, stage, "", rollbackOnFailure, noRollbackOnFailure, bindingEnv, "")
		if err != nil {
			results = append(results, ComposeDeployResult{Service: name, Error: err.Error()})
			return &ComposeDeployResultList{Services: results}, err
		}

		var receipt *state.Receipt
		if ctx, err := Bootstrap(configPath, stage, ""); err == nil {
			receipt, _ = ctx.Backends.Receipts.Load(stage)
		}
		var url string
		if receipt != nil && receipt.Outputs != nil {
			url = ServiceURLFromReceipt(receipt.Outputs)
		}
		if url != "" {
			serviceURLs[name] = url
		}

		outputs := make(map[string]string)
		if receipt != nil && receipt.Outputs != nil {
			outputs = receipt.Outputs
		}
		results = append(results, ComposeDeployResult{
			Service: name,
			Result:  result,
			Outputs: outputs,
		})
	}

	return &ComposeDeployResultList{Services: results}, nil
}

// ComposeRemove removes deployments for all services in the compose file (reverse dependency order).
func ComposeRemove(composePath, stage string) (any, error) {
	c, err := workflow.LoadCompose(composePath)
	if err != nil {
		return nil, err
	}
	configPaths, err := workflow.ResolveServiceConfigPaths(composePath, c)
	if err != nil {
		return nil, err
	}
	order, err := workflow.TopoOrder(c)
	if err != nil {
		return nil, err
	}
	// Remove in reverse order so dependents are removed before dependencies.
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		configPath := configPaths[name]
		if _, err := Remove(configPath, stage, ""); err != nil {
			return nil, fmt.Errorf("remove %q: %w", name, err)
		}
	}
	return map[string]string{"message": "compose remove complete"}, nil
}
