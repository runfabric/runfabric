package runtime

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

// NewBuiltinRegistry returns a runtimes registry populated with built-in runtime plugins.
func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(nodeRuntime{})
	_ = reg.Register(pythonRuntime{})
	return reg
}
