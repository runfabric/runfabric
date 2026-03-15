package config

type Config struct {
	Service   string                    `yaml:"service"`
	Provider  ProviderConfig            `yaml:"provider"`
	Backend   *BackendConfig            `yaml:"backend,omitempty"`
	Functions map[string]FunctionConfig `yaml:"functions"`
	Stages    map[string]StageConfig    `yaml:"stages,omitempty"`
}

type ProviderConfig struct {
	Name    string `yaml:"name"`
	Runtime string `yaml:"runtime"`
	Region  string `yaml:"region,omitempty"`
}

type BackendConfig struct {
	Kind      string `yaml:"kind,omitempty"`
	S3Bucket  string `yaml:"s3Bucket,omitempty"`
	S3Prefix  string `yaml:"s3Prefix,omitempty"`
	LockTable string `yaml:"lockTable,omitempty"`
}

type FunctionConfig struct {
	Handler      string            `yaml:"handler"`
	Runtime      string            `yaml:"runtime,omitempty"`
	Memory       int               `yaml:"memory,omitempty"`
	Timeout      int               `yaml:"timeout,omitempty"`
	Architecture string            `yaml:"architecture,omitempty"`
	Environment  map[string]string `yaml:"environment,omitempty"`
	Tags         map[string]string `yaml:"tags,omitempty"`
	Layers       []string          `yaml:"layers,omitempty"`
	Secrets      map[string]string `yaml:"secrets,omitempty"`
	Events       []EventConfig     `yaml:"events,omitempty"`
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
	Queue   string `yaml:"queue"`             // Queue name or ARN
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
	Bus     string         `yaml:"bus,omitempty"`    // event bus name, default "default"
}

// PubSubEvent configures pub/sub trigger (e.g. GCP Pub/Sub).
type PubSubEvent struct {
	Topic    string `yaml:"topic"`
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
