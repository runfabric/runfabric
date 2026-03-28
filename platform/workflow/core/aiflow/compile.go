package aiflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type NodeInput struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

type EdgeInput struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Expression string `json:"expression,omitempty"`
}

type GraphInput struct {
	Enable     bool        `json:"enable"`
	Entrypoint string      `json:"entrypoint"`
	Nodes      []NodeInput `json:"nodes,omitempty"`
	Edges      []EdgeInput `json:"edges,omitempty"`
}

type CompiledGraph struct {
	Hash       string      `json:"hash"`
	Entrypoint string      `json:"entrypoint"`
	Nodes      []NodeInput `json:"nodes,omitempty"`
	Edges      []EdgeInput `json:"edges,omitempty"`
}

func Compile(in *GraphInput) (*CompiledGraph, error) {
	if in == nil || !in.Enable {
		return nil, nil
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	return &CompiledGraph{
		Hash:       hex.EncodeToString(sum[:]),
		Entrypoint: in.Entrypoint,
		Nodes:      append([]NodeInput(nil), in.Nodes...),
		Edges:      append([]EdgeInput(nil), in.Edges...),
	}, nil
}
