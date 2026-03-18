package providers

import "context"

// providerFromPlugin wraps a ProviderPlugin to implement the legacy Provider interface.
// Used so the app/lifecycle can keep calling p.Doctor(cfg, stage) etc. while the plugin uses the new interface.
type providerFromPlugin struct {
	ProviderPlugin
}

// Name returns the plugin name from Meta().
func (a *providerFromPlugin) Name() string {
	return a.Meta().Name
}

// Doctor forwards to the plugin with a request.
func (a *providerFromPlugin) Doctor(cfg *Config, stage string) (*DoctorResult, error) {
	return a.ProviderPlugin.Doctor(context.Background(), DoctorRequest{Config: cfg, Stage: stage})
}

// Plan forwards to the plugin with a request.
func (a *providerFromPlugin) Plan(cfg *Config, stage, root string) (*PlanResult, error) {
	return a.ProviderPlugin.Plan(context.Background(), PlanRequest{Config: cfg, Stage: stage, Root: root})
}

// Deploy forwards to the plugin with a request.
func (a *providerFromPlugin) Deploy(cfg *Config, stage, root string) (*DeployResult, error) {
	return a.ProviderPlugin.Deploy(context.Background(), DeployRequest{Config: cfg, Stage: stage, Root: root})
}

// Remove forwards to the plugin with a request.
func (a *providerFromPlugin) Remove(cfg *Config, stage, root string) (*RemoveResult, error) {
	return a.ProviderPlugin.Remove(context.Background(), RemoveRequest{Config: cfg, Stage: stage, Root: root})
}

// Invoke forwards to the plugin with a request.
func (a *providerFromPlugin) Invoke(cfg *Config, stage, function string, payload []byte) (*InvokeResult, error) {
	return a.ProviderPlugin.Invoke(context.Background(), InvokeRequest{Config: cfg, Stage: stage, Function: function, Payload: payload})
}

// Logs forwards to the plugin with a request.
func (a *providerFromPlugin) Logs(cfg *Config, stage, function string) (*LogsResult, error) {
	return a.ProviderPlugin.Logs(context.Background(), LogsRequest{Config: cfg, Stage: stage, Function: function})
}

// legacyAdapter wraps a legacy Provider to implement ProviderPlugin so it can be stored in the registry.
type legacyAdapter struct {
	Provider
}

func (a *legacyAdapter) Meta() ProviderMeta {
	return ProviderMeta{
		Name:            a.Name(),
		Capabilities:    []string{"deploy", "remove", "invoke", "logs", "doctor", "plan"},
		SupportsRuntime: []string{"nodejs", "python"},
	}
}

func (a *legacyAdapter) ValidateConfig(ctx context.Context, req ValidateConfigRequest) error {
	return nil
}

func (a *legacyAdapter) Doctor(ctx context.Context, req DoctorRequest) (*DoctorResult, error) {
	return a.Provider.Doctor(req.Config, req.Stage)
}

func (a *legacyAdapter) Plan(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	return a.Provider.Plan(req.Config, req.Stage, req.Root)
}

func (a *legacyAdapter) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	return a.Provider.Deploy(req.Config, req.Stage, req.Root)
}

func (a *legacyAdapter) Remove(ctx context.Context, req RemoveRequest) (*RemoveResult, error) {
	return a.Provider.Remove(req.Config, req.Stage, req.Root)
}

func (a *legacyAdapter) Invoke(ctx context.Context, req InvokeRequest) (*InvokeResult, error) {
	return a.Provider.Invoke(req.Config, req.Stage, req.Function, req.Payload)
}

func (a *legacyAdapter) Logs(ctx context.Context, req LogsRequest) (*LogsResult, error) {
	return a.Provider.Logs(req.Config, req.Stage, req.Function)
}
