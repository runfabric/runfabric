package contracts

import "context"

// namedPlugin wraps a ProviderPlugin to expose a different Meta().Name.
type namedPlugin struct {
	name string
	ProviderPlugin
}

func (n *namedPlugin) Meta() ProviderMeta {
	meta := n.ProviderPlugin.Meta()
	meta.Name = n.name
	return meta
}

func (n *namedPlugin) ValidateConfig(ctx context.Context, req ValidateConfigRequest) error {
	return n.ProviderPlugin.ValidateConfig(ctx, req)
}

func (n *namedPlugin) Doctor(ctx context.Context, req DoctorRequest) (*DoctorResult, error) {
	return n.ProviderPlugin.Doctor(ctx, req)
}

func (n *namedPlugin) Plan(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	return n.ProviderPlugin.Plan(ctx, req)
}

func (n *namedPlugin) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	return n.ProviderPlugin.Deploy(ctx, req)
}

func (n *namedPlugin) Remove(ctx context.Context, req RemoveRequest) (*RemoveResult, error) {
	return n.ProviderPlugin.Remove(ctx, req)
}

func (n *namedPlugin) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	return n.ProviderPlugin.Invoke(ctx, req)
}

func (n *namedPlugin) Logs(ctx context.Context, req LogsRequest) (*LogsResult, error) {
	return n.ProviderPlugin.Logs(ctx, req)
}

// NewNamedPlugin returns a ProviderPlugin that delegates to p but reports Meta().Name as name.
func NewNamedPlugin(name string, p ProviderPlugin) ProviderPlugin {
	return &namedPlugin{name: name, ProviderPlugin: p}
}
