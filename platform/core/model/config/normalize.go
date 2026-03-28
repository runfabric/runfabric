package config

import "strings"

// Normalize fills resolved function/addon/deploy defaults after YAML decode.
// Call after Load so the rest of the code can use cfg.Functions.
func Normalize(cfg *Config) {
	// Functions: from FunctionsConfig.
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
