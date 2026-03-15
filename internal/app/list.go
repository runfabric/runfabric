package app

import "github.com/runfabric/runfabric/internal/state"

// ListResult is the result of List (functions and optional deployment status).
type ListResult struct {
	Service    string         `json:"service"`
	Stage      string         `json:"stage"`
	Provider   string         `json:"provider"`
	Functions  []FunctionInfo `json:"functions"`
	Deployed   bool           `json:"deployed,omitempty"`
	ReceiptAt  string         `json:"receiptAt,omitempty"`
}

// FunctionInfo is one function in the list.
type FunctionInfo struct {
	Name     string `json:"name"`
	Handler  string `json:"handler"`
	Runtime  string `json:"runtime,omitempty"`
	Deployed bool   `json:"deployed,omitempty"`
}

// List returns functions from config and deployment status from receipt (list the code / deployments).
func List(configPath, stage string) (any, error) {
	ctx, err := Bootstrap(configPath, stage)
	if err != nil {
		return nil, err
	}

	receipt, _ := state.Load(ctx.RootDir, ctx.Stage)
	deployedSet := make(map[string]bool)
	var receiptAt string
	if receipt != nil {
		receiptAt = receipt.UpdatedAt
		for _, f := range receipt.Functions {
			deployedSet[f.Function] = true
		}
	}

	functions := make([]FunctionInfo, 0, len(ctx.Config.Functions))
	for name, fn := range ctx.Config.Functions {
		runtime := fn.Runtime
		if runtime == "" {
			runtime = ctx.Config.Provider.Runtime
		}
		functions = append(functions, FunctionInfo{
			Name:     name,
			Handler:  fn.Handler,
			Runtime:  runtime,
			Deployed: deployedSet[name],
		})
	}

	return &ListResult{
		Service:   ctx.Config.Service,
		Stage:     ctx.Stage,
		Provider:  ctx.Config.Provider.Name,
		Functions: functions,
		Deployed:  receipt != nil,
		ReceiptAt: receiptAt,
	}, nil
}
