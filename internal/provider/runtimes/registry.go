package runtimes

import (
	"context"

	provider "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	runtime "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	noderuntime "github.com/runfabric/runfabric/platform/deploy/runtime/node"
	pythonruntime "github.com/runfabric/runfabric/platform/deploy/runtime/python"
	extbuild "github.com/runfabric/runfabric/platform/runtime/build/build"
)

type nodeRuntime struct{}

func (r nodeRuntime) Meta() runtime.Meta {
	return runtime.Meta{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime plugin (build/invoke)"}
}

func (r nodeRuntime) Build(_ context.Context, req runtime.BuildRequest) (*provider.Artifact, error) {
	return extbuild.PackageNodeFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r nodeRuntime) Invoke(_ context.Context, req runtime.InvokeRequest) (*runtime.InvokeResult, error) {
	runner := &noderuntime.Runner{Root: req.Root}
	out, err := runner.Run(req.FunctionConfig.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &runtime.InvokeResult{Output: out}, nil
}

type pythonRuntime struct{}

func (r pythonRuntime) Meta() runtime.Meta {
	return runtime.Meta{ID: "python", Name: "Python", Description: "Python runtime plugin (build/invoke)"}
}

func (r pythonRuntime) Build(_ context.Context, req runtime.BuildRequest) (*provider.Artifact, error) {
	return extbuild.PackagePythonFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r pythonRuntime) Invoke(_ context.Context, req runtime.InvokeRequest) (*runtime.InvokeResult, error) {
	runner := &pythonruntime.Runner{Root: req.Root}
	out, err := runner.Run(req.FunctionConfig.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &runtime.InvokeResult{Output: out}, nil
}

// BuiltinRuntimeManifests returns runtime metadata entries used by extension manifest catalogs.
// It includes both canonical runtime IDs and compatibility aliases accepted by NormalizeRuntimeID.
func BuiltinRuntimeManifests() []runtime.Meta {
	return []runtime.Meta{
		{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime (build and invoke)"},
		{ID: "runtime-node", Name: "Node.js", Description: "Node.js runtime (alias)"},
		{ID: "python", Name: "Python", Description: "Python runtime (build and invoke)"},
		{ID: "runtime-python", Name: "Python", Description: "Python runtime (alias)"},
	}
}

// NewBuiltinRegistry returns a runtime registry pre-populated with built-in
// Node.js and Python runtimes.
func NewBuiltinRegistry() *runtime.Registry {
	reg := runtime.NewRegistry()
	_ = reg.Register(nodeRuntime{})
	_ = reg.Register(pythonRuntime{})
	return reg
}
