package simulators

import (
	"context"
	"encoding/json"

	extrunsim "github.com/runfabric/runfabric/platform/core/contracts/simulators"
)

type localSimulator struct{}

func (s localSimulator) Meta() extrunsim.Meta {
	return extrunsim.Meta{
		ID:          "local",
		Name:        "Local Simulator",
		Description: "Built-in local simulator for call-local/dev workflows",
	}
}

func (s localSimulator) Simulate(_ context.Context, req extrunsim.Request) (*extrunsim.Response, error) {
	body := map[string]any{
		"message":  "invoke local",
		"service":  req.Service,
		"stage":    req.Stage,
		"function": req.Function,
		"method":   req.Method,
		"path":     req.Path,
	}
	if len(req.Query) > 0 {
		body["query"] = req.Query
	}
	if len(req.Headers) > 0 {
		body["headers"] = req.Headers
	}
	if len(req.Body) > 0 {
		body["body"] = string(req.Body)
	}
	raw, _ := json.Marshal(body)
	return &extrunsim.Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: raw,
	}, nil
}

// NewBuiltinSimulatorsRegistry returns a simulator registry populated with built-in simulators.
func NewBuiltinSimulatorsRegistry() *extrunsim.Registry {
	reg := extrunsim.NewRegistry()
	_ = reg.Register(localSimulator{})
	return reg
}
