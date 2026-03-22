package config

import "strings"

// Normalize fills Provider, Backend, and Functions from reference-format fields (providers, triggers, entry, state, functions array).
// Call after Load so the rest of the code can use cfg.Provider, cfg.Backend, cfg.Functions.
func Normalize(cfg *Config) {
	// Provider: from providers[] + runtime when reference format is used
	if len(cfg.Providers) > 0 && cfg.Provider.Name == "" {
		cfg.Provider.Name = cfg.Providers[0]
		if cfg.Runtime != "" {
			cfg.Provider.Runtime = cfg.Runtime
		}
	}

	// Backend: from state when reference format is used
	if cfg.State != nil && cfg.Backend == nil {
		cfg.Backend = stateToBackend(cfg.State)
	}

	// Functions: from FunctionsConfig or from entry + triggers
	if cfg.Functions == nil {
		cfg.Functions = make(map[string]FunctionConfig)
	}
	if len(cfg.FunctionsConfig) > 0 {
		for _, fo := range cfg.FunctionsConfig {
			fn := functionOverrideToConfig(fo, cfg.Provider.Runtime)
			name := fo.Name
			if name == "" {
				name = "api"
			}
			cfg.Functions[name] = fn
		}
	}
	// Reference minimum: entry + triggers (single default function)
	if len(cfg.Functions) == 0 && cfg.Entry != "" && len(cfg.Triggers) > 0 {
		events := make([]EventConfig, 0, len(cfg.Triggers))
		for _, t := range cfg.Triggers {
			if ev := triggerRefToEvent(t); ev != nil {
				events = append(events, *ev)
			}
		}
		cfg.Functions["api"] = FunctionConfig{
			Handler: cfg.Entry,
			Runtime: cfg.Provider.Runtime,
			Events:  events,
		}
		if cfg.Provider.Runtime == "" && cfg.Runtime != "" {
			cfg.Provider.Runtime = cfg.Runtime
			fn := cfg.Functions["api"]
			fn.Runtime = cfg.Runtime
			cfg.Functions["api"] = fn
		}
	}

	// Addons: default name to map key when empty
	if len(cfg.Addons) > 0 {
		for key, addon := range cfg.Addons {
			if addon.Name == "" {
				addon.Name = key
				cfg.Addons[key] = addon
			}
		}
	}

	// Deploy: default strategy to all-at-once; normalize to lowercase
	if cfg.Deploy != nil {
		s := strings.TrimSpace(strings.ToLower(cfg.Deploy.Strategy))
		if s == "" {
			cfg.Deploy.Strategy = "all-at-once"
		} else {
			cfg.Deploy.Strategy = s
		}
	}
}

func stateToBackend(s *StateConfig) *BackendConfig {
	b := &BackendConfig{}
	kind := s.Backend
	if kind == "" {
		kind = "local"
	}
	b.Kind = kind
	switch kind {
	case "s3":
		if s.S3 != nil {
			b.S3Bucket = s.S3.Bucket
			b.S3Prefix = s.S3.KeyPrefix
		}
	case "gcs":
		if s.GCS != nil {
			b.S3Prefix = s.GCS.Prefix
		}
	case "postgres":
		if s.Postgres != nil {
			b.PostgresTable = s.Postgres.Table
			if b.PostgresTable == "" {
				b.PostgresTable = "runfabric_receipts"
			}
			if s.Postgres.ConnectionStringEnv != "" {
				b.PostgresConnectionStringEnv = s.Postgres.ConnectionStringEnv
			} else {
				b.PostgresConnectionStringEnv = "RUNFABRIC_STATE_POSTGRES_URL"
			}
		}
	case "sqlite":
		if s.Local != nil && s.Local.Dir != "" {
			b.SqlitePath = s.Local.Dir + "/state.db"
		}
	case "azblob", "local":
		// no extra fields required
	}
	return b
}

func functionOverrideToConfig(fo FunctionOverrideConfig, defaultRuntime string) FunctionConfig {
	fn := FunctionConfig{
		Handler:     fo.Entry,
		Runtime:     fo.Runtime,
		Environment: fo.Env,
		Addons:      fo.Addons,
	}
	if fn.Runtime == "" {
		fn.Runtime = defaultRuntime
	}
	if fn.Handler == "" {
		fn.Handler = fo.Name
	}
	for _, t := range fo.Triggers {
		if ev := triggerRefToEvent(t); ev != nil {
			fn.Events = append(fn.Events, *ev)
		}
	}
	return fn
}

func triggerRefToEvent(t TriggerRef) *EventConfig {
	ev := &EventConfig{}
	switch t.Type {
	case "http":
		ev.HTTP = &HTTPEvent{Method: t.Method, Path: t.Path}
		if ev.HTTP.Method == "" {
			ev.HTTP.Method = "GET"
		}
	case "cron":
		ev.Cron = t.Schedule
	case "queue":
		ev.Queue = &QueueEvent{Queue: t.Queue, Batch: t.BatchSize}
		ev.Queue.Enabled = t.Enabled
	case "storage":
		ev.Storage = &StorageEvent{Bucket: t.Bucket, Prefix: t.Prefix, Suffix: t.Suffix, Events: t.Events}
	case "eventbridge":
		ev.EventBridge = &EventBridgeEvent{Pattern: t.Pattern, Bus: t.Bus}
	case "pubsub":
		ev.PubSub = &PubSubEvent{Topic: t.Topic, Subscription: t.Subscription}
	case "kafka":
		ev.Kafka = &KafkaEvent{BootstrapServers: t.Brokers, Topic: t.Topic, GroupID: t.GroupID}
	case "rabbitmq":
		ev.RabbitMQ = &RabbitMQEvent{Queue: t.Queue, URL: ""} // URL from env or extension in reference format
	default:
		return nil
	}
	return ev
}
