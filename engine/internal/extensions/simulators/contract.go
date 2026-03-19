package simulators

import (
	"context"
	"encoding/json"
)

type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type Request struct {
	Service  string            `json:"service,omitempty"`
	Stage    string            `json:"stage,omitempty"`
	Function string            `json:"function,omitempty"`
	Method   string            `json:"method,omitempty"`
	Path     string            `json:"path,omitempty"`
	Query    map[string]string `json:"query,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     []byte            `json:"body,omitempty"`
}

type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       json.RawMessage   `json:"body,omitempty"`
}

type Simulator interface {
	Meta() Meta
	Simulate(ctx context.Context, req Request) (*Response, error)
}
