package runtimes

import (
	"context"

	extproviders "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	extruntimes "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	noderuntime "github.com/runfabric/runfabric/platform/deploy/runtime/node"
	pythonruntime "github.com/runfabric/runfabric/platform/deploy/runtime/python"
	extbuild "github.com/runfabric/runfabric/platform/runtime/build/build"
)

// nodeRuntime is the built-in Node.js runtime implementation.
type nodeRuntime struct{}

func (r nodeRuntime) Meta() extruntimes.Meta {
	return extruntimes.Meta{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime plugin (build/invoke)"}
}

func (r nodeRuntime) Build(_ context.Context, req extruntimes.BuildRequest) (*extproviders.Artifact, error) {
	return extbuild.PackageNodeFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r nodeRuntime) Invoke(_ context.Context, req extruntimes.InvokeRequest) (*extruntimes.InvokeResult, error) {
	runner := &noderuntime.Runner{Root: req.Root}
	out, err := runner.Run(req.FunctionConfig.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &extruntimes.InvokeResult{Output: out}, nil
}

// pythonRuntime is the built-in Python runtime implementation.
type pythonRuntime struct{}

func (r pythonRuntime) Meta() extruntimes.Meta {
	return extruntimes.Meta{ID: "python", Name: "Python", Description: "Python runtime plugin (build/invoke)"}
}

func (r pythonRuntime) Build(_ context.Context, req extruntimes.BuildRequest) (*extproviders.Artifact, error) {
	return extbuild.PackagePythonFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r pythonRuntime) Invoke(_ context.Context, req extruntimes.InvokeRequest) (*extruntimes.InvokeResult, error) {
	runner := &pythonruntime.Runner{Root: req.Root}
	out, err := runner.Run(req.FunctionConfig.Handler, req.Payload)
	if err != nil {
		return nil, err
	}
	return &extruntimes.InvokeResult{Output: out}, nil
}

// NewBuiltinRegistry returns a runtime registry populated with built-in runtimes.
func NewBuiltinRegistry() *extruntimes.Registry {
	reg := extruntimes.NewRegistry()
	_ = reg.Register(nodeRuntime{})
	_ = reg.Register(pythonRuntime{})
	return reg
}
