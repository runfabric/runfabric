package config

type Config struct {
	Service  string         `yaml:"service"`
	Provider ProviderConfig `yaml:"provider"`
	Backend  *BackendConfig `yaml:"backend,omitempty"`
	// App and Org: optional grouping for dashboard or multi-service UI (Phase 3.3).
	App string `yaml:"app,omitempty"`
	Org string `yaml:"org,omitempty"`

	// ProviderOverrides: multi-cloud. Key = logical name for --provider (e.g. aws, gcp). When set and CLI passes --provider X, Provider is replaced by ProviderOverrides[X].
	ProviderOverrides map[string]ProviderConfig `yaml:"providerOverrides,omitempty"`

	// Reference-format fields (see docs runfabric.yml reference). When set, Normalize() fills Provider/Backend/Functions.
	Runtime    string            `yaml:"runtime,omitempty"`
	Entry      string            `yaml:"entry,omitempty"`
	Providers  []string          `yaml:"providers,omitempty"`
	Triggers   []TriggerRef      `yaml:"triggers,omitempty"`
	State      *StateConfig      `yaml:"state,omitempty"`
	Deploy     *DeployConfig     `yaml:"deploy,omitempty"`
	Extensions map[string]any    `yaml:"extensions,omitempty"`
	Hooks      []string          `yaml:"hooks,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Params     map[string]string `yaml:"params,omitempty"`
	Resources  map[string]any    `yaml:"resources,omitempty"`
	Secrets    map[string]string `yaml:"secrets,omitempty"`
	// Integrations / Policies are optional target-model workflow runtime configuration blocks.
	Integrations map[string]any   `yaml:"integrations,omitempty"`
	Policies     map[string]any   `yaml:"policies,omitempty"`
	Workflows    []WorkflowConfig `yaml:"workflows,omitempty"`
	// FunctionsConfig stores reference-format function overrides. Normalize() resolves them into Functions.
	FunctionsConfig []FunctionOverrideConfig  `yaml:"functions,omitempty"`
	Functions       map[string]FunctionConfig `yaml:"-"` // resolved by Normalize(); use this everywhere
	Stages          map[string]StageConfig    `yaml:"stages,omitempty"`
	// Layers: first-class layer declarations. Key = logical name; value = provider-specific ref. Functions reference by name (e.g. layers: ["node-deps"]) and resolver expands to the concrete ref.
	Layers map[string]LayerConfig `yaml:"layers,omitempty"`
	// Addons: marketplace/add-on declarations. Key = logical name; secrets binding injects env vars at deploy. See RUNFABRIC_YML_REFERENCE addons section.
	Addons          map[string]AddonConfig `yaml:"addons,omitempty"`
	AddonCatalogURL string                 `yaml:"addonCatalogUrl,omitempty"` // optional URL to fetch catalog entries (JSON array); merged with built-in for addons list
	// Fabric: runtime fabric for active-active deploy, failover, and latency-based routing. Targets = provider keys (from providerOverrides) to deploy to.
	Fabric *FabricConfig `yaml:"fabric,omitempty"`
	// Logs: optional config for unified log source (1.7). Path = directory under project root for local log files (e.g. .runfabric/logs). When set, runfabric logs also reads from local files <path>/<stage>.log and <path>/<function>_<stage>.log.
	Logs *LogsConfig `yaml:"logs,omitempty"`
	// Build: optional build-step ordering (Phase 3.1). Order defines execution order of hooks or build steps when multiple are present.
	Build *BuildConfig `yaml:"build,omitempty"`
	// Alerts: optional alerting config (webhook/slack URLs and triggers), used by integrations.
	Alerts *AlertsConfig `yaml:"alerts,omitempty"`
}

// BuildConfig defines optional build-step ordering (e.g. for hooks or plugins). Order lists step or hook identifiers in execution order.
type BuildConfig struct {
	Order []string `yaml:"order,omitempty"` // execution order of build steps or hook modules (e.g. ["deps", "compile", "bundle"])
}

// AlertsConfig defines optional alerting. URLs can use ${env:VAR}. OnError/OnTimeout enable triggers; delivery is integration-specific.
type AlertsConfig struct {
	Webhook   string `yaml:"webhook,omitempty"`   // HTTP POST URL for alerts (e.g. errors, timeouts)
	Slack     string `yaml:"slack,omitempty"`     // Slack webhook URL
	OnError   bool   `yaml:"onError,omitempty"`   // trigger on function errors
	OnTimeout bool   `yaml:"onTimeout,omitempty"` // trigger on function timeouts
}

// LogsConfig configures optional local log file source (unified with provider logs).
type LogsConfig struct {
	Path string `yaml:"path,omitempty"` // directory for local log files relative to project root (default .runfabric/logs)
}

// FabricConfig configures the runtime fabric (active-active, failover, latency routing). Requires providerOverrides.
type FabricConfig struct {
	Targets     []string           `yaml:"targets,omitempty"`     // provider keys to deploy to (e.g. ["aws-us", "aws-eu"])
	HealthCheck *HealthCheckConfig `yaml:"healthCheck,omitempty"` // optional health check for fabric endpoints
	Routing     string             `yaml:"routing,omitempty"`     // "failover" | "latency" | "round-robin"; used when configuring DNS/LB
}

// AddonConfig declares an add-on (e.g. Sentry, Datadog). Options are addon-specific. Secrets maps env var names to refs (e.g. ${env:VAR} or key in config.Secrets); resolved at deploy and injected into function environment.
type AddonConfig struct {
	Name    string            `yaml:"name,omitempty"`    // optional; defaults to map key
	Version string            `yaml:"version,omitempty"` // optional semver or tag
	Options map[string]any    `yaml:"options,omitempty"` // addon-specific config
	Secrets map[string]string `yaml:"secrets,omitempty"` // env var name -> secret ref (${env:VAR} or key in top-level secrets)
}

// LayerConfig defines a named layer. Ref is the provider-specific layer identifier; Arn is a deprecated AWS-specific alias kept for compatibility.
type LayerConfig struct {
	Ref     string `yaml:"ref,omitempty"`
	Arn     string `yaml:"arn,omitempty"`
	Name    string `yaml:"name,omitempty"`
	Version string `yaml:"version,omitempty"`
}

// TriggerRef is the reference-format trigger: type + type-specific fields (http: method, path; cron: schedule; queue: queue; etc.).
type TriggerRef struct {
	Type                         string         `yaml:"type"`
	HTTP                         *TriggerHTTP   `yaml:"-"` // populated when type==http from top-level method, path
	Method                       string         `yaml:"method,omitempty"`
	Path                         string         `yaml:"path,omitempty"`
	Schedule                     string         `yaml:"schedule,omitempty"`
	Timezone                     string         `yaml:"timezone,omitempty"`
	Queue                        string         `yaml:"queue,omitempty"`
	BatchSize                    int            `yaml:"batchSize,omitempty"`
	MaximumBatchingWindowSeconds int            `yaml:"maximumBatchingWindowSeconds,omitempty"`
	MaximumConcurrency           int            `yaml:"maximumConcurrency,omitempty"`
	Enabled                      *bool          `yaml:"enabled,omitempty"`
	FunctionResponseType         string         `yaml:"functionResponseType,omitempty"`
	Bucket                       string         `yaml:"bucket,omitempty"`
	Events                       []string       `yaml:"events,omitempty"`
	Prefix                       string         `yaml:"prefix,omitempty"`
	Suffix                       string         `yaml:"suffix,omitempty"`
	ExistingBucket               *bool          `yaml:"existingBucket,omitempty"`
	Pattern                      map[string]any `yaml:"pattern,omitempty"`
	Bus                          string         `yaml:"bus,omitempty"`
	Topic                        string         `yaml:"topic,omitempty"`
	Subscription                 string         `yaml:"subscription,omitempty"`
	Brokers                      []string       `yaml:"brokers,omitempty"`
	GroupID                      string         `yaml:"groupId,omitempty"`
	Exchange                     string         `yaml:"exchange,omitempty"`
	RoutingKey                   string         `yaml:"routingKey,omitempty"`
}

type TriggerHTTP struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
}

// StateConfig is the reference-format state backend (state.backend, state.s3, state.lock, etc.).
type StateConfig struct {
	Backend   string           `yaml:"backend,omitempty"`
	KeyPrefix string           `yaml:"keyPrefix,omitempty"`
	Lock      *StateLockConfig `yaml:"lock,omitempty"`
	Local     *StateLocal      `yaml:"local,omitempty"`
	Postgres  *StatePostgres   `yaml:"postgres,omitempty"`
	S3        *StateS3         `yaml:"s3,omitempty"`
	GCS       *StateGCS        `yaml:"gcs,omitempty"`
	Azblob    *StateAzblob     `yaml:"azblob,omitempty"`
}

type StateLockConfig struct {
	Enabled           bool `yaml:"enabled,omitempty"`
	TimeoutSeconds    int  `yaml:"timeoutSeconds,omitempty"`
	HeartbeatSeconds  int  `yaml:"heartbeatSeconds,omitempty"`
	StaleAfterSeconds int  `yaml:"staleAfterSeconds,omitempty"`
}

type StateLocal struct {
	Dir string `yaml:"dir,omitempty"`
}

type StatePostgres struct {
	ConnectionStringEnv string `yaml:"connectionStringEnv,omitempty"`
	Schema              string `yaml:"schema,omitempty"`
	Table               string `yaml:"table,omitempty"`
}

type StateS3 struct {
	Bucket      string `yaml:"bucket,omitempty"`
	Region      string `yaml:"region,omitempty"`
	KeyPrefix   string `yaml:"keyPrefix,omitempty"`
	UseLockfile bool   `yaml:"useLockfile,omitempty"`
}

type StateGCS struct {
	Bucket string `yaml:"bucket,omitempty"`
	Prefix string `yaml:"prefix,omitempty"`
}

type StateAzblob struct {
	Container string `yaml:"container,omitempty"`
	Prefix    string `yaml:"prefix,omitempty"`
}

// DeployConfig holds deploy policy (e.g. rollbackOnFailure), optional health check, scaling defaults, and canary/blue-green strategy.
type DeployConfig struct {
	RollbackOnFailure     *bool              `yaml:"rollbackOnFailure,omitempty"`
	HealthCheck           *HealthCheckConfig `yaml:"healthCheck,omitempty"`
	Scaling               *ScalingConfig     `yaml:"scaling,omitempty"`
	Strategy              string             `yaml:"strategy,omitempty"`              // all-at-once (default), canary, blue-green
	CanaryPercent         int                `yaml:"canaryPercent,omitempty"`         // 0-100 when strategy=canary
	CanaryIntervalMinutes int                `yaml:"canaryIntervalMinutes,omitempty"` // minutes before full shift when strategy=canary
}

// HealthCheckConfig is optional post-deploy health check (HTTP GET). If URL is empty, the deployed URL from receipt/outputs is used.
type HealthCheckConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	URL     string `yaml:"url,omitempty"`
}

// ScalingConfig holds provider-level scaling defaults (can be overridden per function).
type ScalingConfig struct {
	ReservedConcurrency    int `yaml:"reservedConcurrency,omitempty"`
	ProvisionedConcurrency int `yaml:"provisionedConcurrency,omitempty"`
}

// WorkflowConfig is a workflow definition (name, steps).
type WorkflowConfig struct {
	Name  string         `yaml:"name"`
	Steps []WorkflowStep `yaml:"steps,omitempty"`
}

type WorkflowStep struct {
	Function string         `yaml:"function,omitempty"`
	Next     string         `yaml:"next,omitempty"`
	Retry    *WorkflowRetry `yaml:"retry,omitempty"`
}

type WorkflowRetry struct {
	Attempts       int `yaml:"attempts,omitempty"`
	BackoffSeconds int `yaml:"backoffSeconds,omitempty"`
}

// FunctionOverrideConfig is the reference-format function (array element: name, entry, runtime, triggers, env, addons).
type FunctionOverrideConfig struct {
	Name     string            `yaml:"name"`
	Entry    string            `yaml:"entry,omitempty"`
	Runtime  string            `yaml:"runtime,omitempty"`
	Triggers []TriggerRef      `yaml:"triggers,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Addons   []string          `yaml:"addons,omitempty"`
}

type ProviderConfig struct {
	Name    string         `yaml:"name"`
	Runtime string         `yaml:"runtime"`
	Region  string         `yaml:"region,omitempty"`
	Source  string         `yaml:"source,omitempty"`  // builtin (default) | external
	Version string         `yaml:"version,omitempty"` // external plugin version pin
	Backend *BackendConfig `yaml:"backend,omitempty"` // optional per-provider state backend (e.g. s3 for aws, gcs for gcp)
}

type BackendConfig struct {
	Kind      string `yaml:"kind,omitempty"`
	S3Bucket  string `yaml:"s3Bucket,omitempty"`
	S3Prefix  string `yaml:"s3Prefix,omitempty"`
	LockTable string `yaml:"lockTable,omitempty"`
	// DB-backed deploy state (receipts): 1.5 / 1.6
	PostgresConnectionStringEnv string `yaml:"postgresConnectionStringEnv,omitempty"` // env var for DSN when kind=postgres
	PostgresTable               string `yaml:"postgresTable,omitempty"`               // table name for receipts (default runfabric_receipts)
	SqlitePath                  string `yaml:"sqlitePath,omitempty"`                  // path to SQLite file; used when kind=sqlite
	ReceiptTable                string `yaml:"receiptTable,omitempty"`                // DynamoDB table for receipts when kind=dynamodb; if empty, use lockTable
}

type FunctionConfig struct {
	Handler                string            `yaml:"handler"`
	Runtime                string            `yaml:"runtime,omitempty"`
	Memory                 int               `yaml:"memory,omitempty"`
	Timeout                int               `yaml:"timeout,omitempty"`
	Architecture           string            `yaml:"architecture,omitempty"`
	Environment            map[string]string `yaml:"environment,omitempty"`
	Tags                   map[string]string `yaml:"tags,omitempty"`
	Layers                 []string          `yaml:"layers,omitempty"`
	Resources              []string          `yaml:"resources,omitempty"` // optional: only these top-level resource keys are injected; if empty, all resources apply
	Addons                 []string          `yaml:"addons,omitempty"`    // optional: only these addon keys get their secrets injected; if empty, all addons apply
	ReservedConcurrency    int               `yaml:"reservedConcurrency,omitempty"`
	ProvisionedConcurrency int               `yaml:"provisionedConcurrency,omitempty"`
	Secrets                map[string]string `yaml:"secrets,omitempty"`
	Events                 []EventConfig     `yaml:"events,omitempty"`
}

type EventConfig struct {
	HTTP        *HTTPEvent        `yaml:"http,omitempty"`
	Cron        string            `yaml:"cron,omitempty"`
	Queue       *QueueEvent       `yaml:"queue,omitempty"`
	Storage     *StorageEvent     `yaml:"storage,omitempty"`
	EventBridge *EventBridgeEvent `yaml:"eventbridge,omitempty"`
	PubSub      *PubSubEvent      `yaml:"pubsub,omitempty"`
	Kafka       *KafkaEvent       `yaml:"kafka,omitempty"`
	RabbitMQ    *RabbitMQEvent    `yaml:"rabbitmq,omitempty"`
}

// QueueEvent configures a queue trigger (e.g. SQS, Azure Queue, GCP Task Queue).
type QueueEvent struct {
	Queue   string `yaml:"queue"` // Queue name or ARN
	Batch   int    `yaml:"batchSize,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty"`
}

// StorageEvent configures object storage trigger (e.g. S3, GCS, Azure Blob).
type StorageEvent struct {
	Bucket string   `yaml:"bucket"`
	Prefix string   `yaml:"prefix,omitempty"`
	Suffix string   `yaml:"suffix,omitempty"`
	Events []string `yaml:"events,omitempty"` // e.g. s3:ObjectCreated:*, s3:ObjectRemoved:*
}

// EventBridgeEvent configures AWS EventBridge (or equivalent) rule.
type EventBridgeEvent struct {
	Pattern map[string]any `yaml:"pattern,omitempty"` // event pattern
	Bus     string         `yaml:"bus,omitempty"`     // event bus name, default "default"
}

// PubSubEvent configures pub/sub trigger (e.g. GCP Pub/Sub).
type PubSubEvent struct {
	Topic        string `yaml:"topic"`
	Subscription string `yaml:"subscription,omitempty"`
}

// KafkaEvent configures Kafka trigger.
type KafkaEvent struct {
	BootstrapServers []string `yaml:"bootstrapServers"`
	Topic            string   `yaml:"topic"`
	GroupID          string   `yaml:"groupId,omitempty"`
}

// RabbitMQEvent configures RabbitMQ trigger.
type RabbitMQEvent struct {
	URL   string `yaml:"url"`
	Queue string `yaml:"queue"`
}

type HTTPEvent struct {
	Path          string            `yaml:"path"`
	Method        string            `yaml:"method"`
	CORS          *CORSConfig       `yaml:"cors,omitempty"`
	Authorizer    *AuthorizerConfig `yaml:"authorizer,omitempty"`
	RouteSettings *RouteSettings    `yaml:"routeSettings,omitempty"`
}

type CORSConfig struct {
	AllowOrigins     []string `yaml:"allowOrigins,omitempty"`
	AllowMethods     []string `yaml:"allowMethods,omitempty"`
	AllowHeaders     []string `yaml:"allowHeaders,omitempty"`
	ExposeHeaders    []string `yaml:"exposeHeaders,omitempty"`
	AllowCredentials bool     `yaml:"allowCredentials,omitempty"`
	MaxAge           int      `yaml:"maxAge,omitempty"`
}

type AuthorizerConfig struct {
	Type            string   `yaml:"type"`
	Name            string   `yaml:"name,omitempty"`
	IdentitySources []string `yaml:"identitySources,omitempty"`
	Issuer          string   `yaml:"issuer,omitempty"`
	Audience        []string `yaml:"audience,omitempty"`
	Function        string   `yaml:"function,omitempty"`
}

type RouteSettings struct {
	ThrottlingBurstLimit int `yaml:"throttlingBurstLimit,omitempty"`
	ThrottlingRateLimit  int `yaml:"throttlingRateLimit,omitempty"`
}

type StageHTTPConfig struct {
	Name          string            `yaml:"name,omitempty"`
	Tags          map[string]string `yaml:"tags,omitempty"`
	Domain        *DomainConfig     `yaml:"domain,omitempty"`
	AccessLogging bool              `yaml:"accessLogging,omitempty"`
}

type DomainConfig struct {
	Name        string `yaml:"name"`
	Certificate string `yaml:"certificate,omitempty"`
	BasePath    string `yaml:"basePath,omitempty"`
}

type StageConfig struct {
	Provider  *ProviderConfig           `yaml:"provider,omitempty"`
	Backend   *BackendConfig            `yaml:"backend,omitempty"`
	HTTP      *StageHTTPConfig          `yaml:"http,omitempty"`
	Functions map[string]FunctionConfig `yaml:"functions,omitempty"`
}
