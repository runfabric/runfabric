package runtimes

import (
	"context"

	extproviders "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	noderuntime "github.com/runfabric/runfabric/platform/deploy/runtime/node"
	pythonruntime "github.com/runfabric/runfabric/platform/deploy/runtime/python"
	runtimebuild "github.com/runfabric/runfabric/platform/runtime/build/build"
)

type nodeRuntime struct{}

func (r nodeRuntime) Meta() Meta {
	return Meta{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime plugin (build/invoke)"}
}

func (r nodeRuntime) Build(_ context.Context, req BuildRequest) (*extproviders.Artifact, error) {
	return runtimebuild.PackageNodeFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r nodeRuntime) Invoke(_ context.Context, req InvokeRequest) (*InvokeResult, error) {
	runner := &noderuntime.Runner{Root: req.Root}
	out, err := runner.Run(req.FunctionConfig.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &InvokeResult{Output: out}, nil
}

type pythonRuntime struct{}

func (r pythonRuntime) Meta() Meta {
	return Meta{ID: "python", Name: "Python", Description: "Python runtime plugin (build/invoke)"}
}

func (r pythonRuntime) Build(_ context.Context, req BuildRequest) (*extproviders.Artifact, error) {
	return runtimebuild.PackagePythonFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r pythonRuntime) Invoke(_ context.Context, req InvokeRequest) (*InvokeResult, error) {
	runner := &pythonruntime.Runner{Root: req.Root}
	out, err := runner.Run(req.FunctionConfig.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &InvokeResult{Output: out}, nil
}

type runtimeFactory struct {
	id     string
	create func() Runtime
}

func builtinRuntimeFactories() []runtimeFactory {
	return []runtimeFactory{
		{id: "nodejs", create: func() Runtime { return nodeRuntime{} }},
		{id: "python", create: func() Runtime { return pythonRuntime{} }},
	}
}

// BuiltinRuntimeManifests returns runtime metadata entries used by extension manifest catalogs.
// It includes both canonical runtime IDs and compatibility aliases accepted by NormalizeRuntimeID.
func BuiltinRuntimeManifests() []Meta {
	return []Meta{
		{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime (build and invoke)"},
		{ID: "runtime-node", Name: "Node.js", Description: "Node.js runtime (alias)"},
		{ID: "python", Name: "Python", Description: "Python runtime (build and invoke)"},
		{ID: "runtime-python", Name: "Python", Description: "Python runtime (alias)"},
	}
}

// NewBuiltinRegistry returns a runtimes registry populated with built-in runtime plugins.
func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	for _, f := range builtinRuntimeFactories() {
		_ = reg.Register(f.create())
	}
	return reg
}
