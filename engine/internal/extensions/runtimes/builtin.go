package runtimes

import (
	"context"
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	runtimebuild "github.com/runfabric/runfabric/engine/internal/extensions/runtime/build"
)

type nodeRuntime struct{}

func (r nodeRuntime) Meta() Meta {
	return Meta{ID: "nodejs", Name: "Node.js", Description: "Node.js runtime plugin (build/invoke)"}
}

func (r nodeRuntime) Build(_ context.Context, req BuildRequest) (*providers.Artifact, error) {
	return runtimebuild.PackageNodeFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r nodeRuntime) Invoke(_ context.Context, req InvokeRequest) (*InvokeResult, error) {
	return nil, fmt.Errorf("runtime %q local invoke is not yet supported by builtin runtime plugins", r.Meta().ID)
}

type pythonRuntime struct{}

func (r pythonRuntime) Meta() Meta {
	return Meta{ID: "python", Name: "Python", Description: "Python runtime plugin (build/invoke)"}
}

func (r pythonRuntime) Build(_ context.Context, req BuildRequest) (*providers.Artifact, error) {
	return runtimebuild.PackagePythonFunction(req.Root, req.FunctionName, req.FunctionConfig, req.ConfigSignature)
}

func (r pythonRuntime) Invoke(_ context.Context, req InvokeRequest) (*InvokeResult, error) {
	return nil, fmt.Errorf("runtime %q local invoke is not yet supported by builtin runtime plugins", r.Meta().ID)
}

func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(nodeRuntime{})
	_ = reg.Register(pythonRuntime{})
	return reg
}
