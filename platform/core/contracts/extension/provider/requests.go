package provider

// ProviderMeta is the plugin metadata returned by ProviderPlugin.Meta().
type ProviderMeta struct {
	Name              string   // e.g. aws, cloudflare, vercel, gcp-functions
	Version           string   // provider implementation version
	PluginVersion     string   // contract/API version
	Capabilities      []string // deploy, remove, invoke, logs, doctor, plan
	SupportsRuntime   []string // nodejs, python, etc.
	SupportsTriggers  []string // http, cron, queue, etc.
	SupportsResources []string // optional resource types
}

// ValidateConfigRequest is the input to ValidateConfig.
type ValidateConfigRequest struct {
	Config *Config
	Stage  string
}

// DoctorRequest is the input to Doctor.
type DoctorRequest struct {
	Config *Config
	Stage  string
}

// PlanRequest is the input to Plan.
type PlanRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// DeployRequest is the input to Deploy.
type DeployRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// RemoveRequest is the input to Remove.
type RemoveRequest struct {
	Config  *Config
	Stage   string
	Root    string
	Receipt interface{} // optional receipt for teardown
}

// InvokeRequest is the input to Invoke.
type InvokeRequest struct {
	Config   *Config
	Stage    string
	Function string
	Payload  []byte
}

// LogsRequest is the input to Logs.
type LogsRequest struct {
	Config   *Config
	Stage    string
	Function string
}

// OrchestrationSyncRequest is the input to SyncOrchestrations.
type OrchestrationSyncRequest struct {
	Config *Config
	Stage  string
	Root   string
	// FunctionResourceByName provides deploy-context resource identifiers keyed by logical function name.
	// Each provider interprets the value according to its own model (e.g. AWS: Lambda ARN;
	// GCP: Cloud Function invoke URL; Azure: Function App resource ID).
	FunctionResourceByName map[string]string
}

// OrchestrationRemoveRequest is the input to RemoveOrchestrations.
type OrchestrationRemoveRequest struct {
	Config *Config
	Stage  string
	Root   string
}

// OrchestrationInvokeRequest is the input to InvokeOrchestration.
type OrchestrationInvokeRequest struct {
	Config  *Config
	Stage   string
	Root    string
	Name    string
	Payload []byte
}

// OrchestrationInspectRequest is the input to InspectOrchestrations.
type OrchestrationInspectRequest struct {
	Config *Config
	Stage  string
	Root   string
}
