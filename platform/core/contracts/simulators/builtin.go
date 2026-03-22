package simulators

import (
	"context"
	"encoding/json"
)

type localSimulator struct{}

func (s localSimulator) Meta() Meta {
	return Meta{
		ID:          "local",
		Name:        "Local Simulator",
		Description: "Built-in local simulator for call-local/dev workflows",
	}
}

func (s localSimulator) Simulate(_ context.Context, req Request) (*Response, error) {
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
	return &Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: raw,
	}, nil
}

// NewBuiltinRegistry returns a simulator registry populated with built-in simulators.
func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(localSimulator{})
	return reg
}
