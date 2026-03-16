package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envPattern = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)(?:,([^}]+))?\}`)

func Resolve(cfg *Config, stage string) (*Config, error) {
	out := *cfg

	var err error
	out.Service, err = resolveEnvStrict(out.Service)
	if err != nil {
		return nil, err
	}
	if out.App != "" {
		out.App, err = resolveEnvStrict(out.App)
		if err != nil {
			return nil, err
		}
	}
	if out.Org != "" {
		out.Org, err = resolveEnvStrict(out.Org)
		if err != nil {
			return nil, err
		}
	}
	out.Provider.Name, err = resolveEnvStrict(out.Provider.Name)
	if err != nil {
		return nil, err
	}
	out.Provider.Runtime, err = resolveEnvStrict(out.Provider.Runtime)
	if err != nil {
		return nil, err
	}
	out.Provider.Region, err = resolveEnvStrict(out.Provider.Region)
	if err != nil {
		return nil, err
	}

	if out.Backend != nil {
		b := *out.Backend
		b.Kind, err = resolveEnvStrict(b.Kind)
		if err != nil {
			return nil, err
		}
		b.S3Bucket, err = resolveEnvStrict(b.S3Bucket)
		if err != nil {
			return nil, err
		}
		b.S3Prefix, err = resolveEnvStrict(b.S3Prefix)
		if err != nil {
			return nil, err
		}
		b.LockTable, err = resolveEnvStrict(b.LockTable)
		if err != nil {
			return nil, err
		}
		out.Backend = &b
	}

	if out.Layers != nil {
		resolvedLayers := make(map[string]LayerConfig, len(out.Layers))
		for k, v := range out.Layers {
			arn, err := resolveEnvStrict(v.Arn)
			if err != nil {
				return nil, err
			}
			name := v.Name
			if name != "" {
				name, err = resolveEnvStrict(name)
				if err != nil {
					return nil, err
				}
			}
			version := v.Version
			if version != "" {
				version, err = resolveEnvStrict(version)
				if err != nil {
					return nil, err
				}
			}
			resolvedLayers[k] = LayerConfig{Arn: arn, Name: name, Version: version}
		}
		out.Layers = resolvedLayers
	}

	if out.Build != nil && len(out.Build.Order) > 0 {
		order := make([]string, 0, len(out.Build.Order))
		for _, s := range out.Build.Order {
			resolved, err := resolveEnvStrict(s)
			if err != nil {
				return nil, err
			}
			order = append(order, resolved)
		}
		out.Build = &BuildConfig{Order: order}
	}

	if out.Alerts != nil {
		a := *out.Alerts
		if a.Webhook != "" {
			a.Webhook, err = resolveEnvStrict(a.Webhook)
			if err != nil {
				return nil, err
			}
		}
		if a.Slack != "" {
			a.Slack, err = resolveEnvStrict(a.Slack)
			if err != nil {
				return nil, err
			}
		}
		out.Alerts = &a
	}

	resolvedFunctions := make(map[string]FunctionConfig, len(out.Functions))
	for name, fn := range out.Functions {
		newFn := fn
		// Deep copy Events so we don't mutate the original config
		if len(fn.Events) > 0 {
			newFn.Events = make([]EventConfig, len(fn.Events))
			for i := range fn.Events {
				newFn.Events[i] = copyEventConfig(fn.Events[i])
			}
		}
		newFn.Handler, err = resolveEnvStrict(fn.Handler)
		if err != nil {
			return nil, err
		}
		newFn.Runtime, err = resolveEnvStrict(fn.Runtime)
		if err != nil {
			return nil, err
		}
		newFn.Architecture, err = resolveEnvStrict(fn.Architecture)
		if err != nil {
			return nil, err
		}

		if fn.Environment != nil {
			newFn.Environment = map[string]string{}
			for k, v := range fn.Environment {
				newFn.Environment[k], err = resolveEnvStrict(v)
				if err != nil {
					return nil, err
				}
			}
		}

		if fn.Secrets != nil {
			newFn.Secrets = map[string]string{}
			for k, v := range fn.Secrets {
				newFn.Secrets[k], err = resolveEnvStrict(v)
				if err != nil {
					return nil, err
				}
			}
		}

		if fn.Tags != nil {
			newFn.Tags = map[string]string{}
			for k, v := range fn.Tags {
				newFn.Tags[k], err = resolveEnvStrict(v)
				if err != nil {
					return nil, err
				}
			}
		}

		if len(fn.Layers) > 0 {
			newFn.Layers = make([]string, 0, len(fn.Layers))
			for _, layer := range fn.Layers {
				arn := layer
				if out.Layers != nil {
					if lc, ok := out.Layers[layer]; ok && lc.Arn != "" {
						arn = lc.Arn
					}
				}
				s, e := resolveEnvStrict(arn)
				if e != nil {
					return nil, e
				}
				newFn.Layers = append(newFn.Layers, s)
			}
		}

		for i := range newFn.Events {
			if newFn.Events[i].HTTP != nil {
				h := newFn.Events[i].HTTP
				h.Path, err = resolveEnvStrict(h.Path)
				if err != nil {
					return nil, err
				}
				h.Method, err = resolveEnvStrict(h.Method)
				if err != nil {
					return nil, err
				}

				if h.Authorizer != nil {
					h.Authorizer.Type, err = resolveEnvStrict(h.Authorizer.Type)
					if err != nil {
						return nil, err
					}
					h.Authorizer.Name, err = resolveEnvStrict(h.Authorizer.Name)
					if err != nil {
						return nil, err
					}
					h.Authorizer.Issuer, err = resolveEnvStrict(h.Authorizer.Issuer)
					if err != nil {
						return nil, err
					}
					h.Authorizer.Function, err = resolveEnvStrict(h.Authorizer.Function)
					if err != nil {
						return nil, err
					}
					for j := range h.Authorizer.IdentitySources {
						h.Authorizer.IdentitySources[j], err = resolveEnvStrict(h.Authorizer.IdentitySources[j])
						if err != nil {
							return nil, err
						}
					}
					for j := range h.Authorizer.Audience {
						h.Authorizer.Audience[j], err = resolveEnvStrict(h.Authorizer.Audience[j])
						if err != nil {
							return nil, err
						}
					}
				}
			}
		}

		resolvedFunctions[name] = newFn
	}

	out.Functions = resolvedFunctions

	if stage != "" && out.Stages != nil {
		if stageCfg, ok := out.Stages[stage]; ok {

			if stageCfg.Provider != nil {
				if stageCfg.Provider.Name != "" {
					out.Provider.Name, err = resolveEnvStrict(stageCfg.Provider.Name)
					if err != nil {
						return nil, err
					}
				}
				if stageCfg.Provider.Runtime != "" {
					out.Provider.Runtime, err = resolveEnvStrict(stageCfg.Provider.Runtime)
					if err != nil {
						return nil, err
					}
				}
				if stageCfg.Provider.Region != "" {
					out.Provider.Region, err = resolveEnvStrict(stageCfg.Provider.Region)
					if err != nil {
						return nil, err
					}
				}
			}

			if stageCfg.Backend != nil {
				if out.Backend == nil {
					out.Backend = &BackendConfig{}
				}
				if stageCfg.Backend.Kind != "" {
					out.Backend.Kind, err = resolveEnvStrict(stageCfg.Backend.Kind)
					if err != nil {
						return nil, err
					}
				}
				if stageCfg.Backend.S3Bucket != "" {
					out.Backend.S3Bucket, err = resolveEnvStrict(stageCfg.Backend.S3Bucket)
					if err != nil {
						return nil, err
					}
				}
				if stageCfg.Backend.S3Prefix != "" {
					out.Backend.S3Prefix, err = resolveEnvStrict(stageCfg.Backend.S3Prefix)
					if err != nil {
						return nil, err
					}
				}
				if stageCfg.Backend.LockTable != "" {
					out.Backend.LockTable, err = resolveEnvStrict(stageCfg.Backend.LockTable)
					if err != nil {
						return nil, err
					}
				}
			}

			if stageCfg.HTTP != nil {
				// merge into a stage-aware runtime representation if you already have one
				// or preserve on out.Stages[stage]
			}
			for fnName, fnOverride := range stageCfg.Functions {
				base := out.Functions[fnName]
				if fnOverride.Handler != "" {
					base.Handler, err = resolveEnvStrict(fnOverride.Handler)
					if err != nil {
						return nil, err
					}
				}
				if fnOverride.Runtime != "" {
					base.Runtime, err = resolveEnvStrict(fnOverride.Runtime)
					if err != nil {
						return nil, err
					}
				}
				if len(fnOverride.Events) > 0 {
					base.Events = fnOverride.Events
				}
				out.Functions[fnName] = base
			}
		}
	}

	for name, fn := range out.Functions {
		if fn.Runtime == "" {
			fn.Runtime = out.Provider.Runtime
		}
		if out.Deploy != nil && out.Deploy.Scaling != nil {
			if fn.ReservedConcurrency == 0 && out.Deploy.Scaling.ReservedConcurrency > 0 {
				fn.ReservedConcurrency = out.Deploy.Scaling.ReservedConcurrency
			}
			if fn.ProvisionedConcurrency == 0 && out.Deploy.Scaling.ProvisionedConcurrency > 0 {
				fn.ProvisionedConcurrency = out.Deploy.Scaling.ProvisionedConcurrency
			}
		}
		out.Functions[name] = fn
	}

	return &out, nil
}

// resolveEnvStrict resolves ${env:VAR} and ${env:VAR,default}. Returns error if VAR is unset and no default given.
func resolveEnvStrict(input string) (string, error) {
	var firstErr error
	out := envPattern.ReplaceAllStringFunc(input, func(match string) string {
		sub := envPattern.FindStringSubmatch(match)
		key := sub[1]
		def := ""
		if len(sub) > 2 {
			def = strings.TrimSpace(sub[2])
		}
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		if firstErr == nil && def == "" {
			firstErr = fmt.Errorf("config references ${env:%s} but %s is not set and no default provided", key, key)
		}
		return def
	})
	return out, firstErr
}

func EnsureStage(stage string) error {
	if strings.TrimSpace(stage) == "" {
		return fmt.Errorf("stage cannot be empty")
	}
	return nil
}

// copyEventConfig returns a deep copy of EventConfig so Resolve does not mutate the original config.
func copyEventConfig(e EventConfig) EventConfig {
	out := EventConfig{
		Cron: e.Cron,
	}
	if e.HTTP != nil {
		out.HTTP = &HTTPEvent{
			Path:   e.HTTP.Path,
			Method: e.HTTP.Method,
		}
		if e.HTTP.Authorizer != nil {
			ac := *e.HTTP.Authorizer
			ac.IdentitySources = append([]string(nil), ac.IdentitySources...)
			ac.Audience = append([]string(nil), ac.Audience...)
			out.HTTP.Authorizer = &ac
		}
	}
	if e.Queue != nil {
		out.Queue = &QueueEvent{Queue: e.Queue.Queue, Batch: e.Queue.Batch}
		if e.Queue.Enabled != nil {
			v := *e.Queue.Enabled
			out.Queue.Enabled = &v
		}
	}
	if e.Storage != nil {
		ev := append([]string(nil), e.Storage.Events...)
		out.Storage = &StorageEvent{Bucket: e.Storage.Bucket, Prefix: e.Storage.Prefix, Suffix: e.Storage.Suffix, Events: ev}
	}
	if e.EventBridge != nil {
		out.EventBridge = &EventBridgeEvent{Pattern: e.EventBridge.Pattern, Bus: e.EventBridge.Bus}
	}
	if e.PubSub != nil {
		out.PubSub = &PubSubEvent{Topic: e.PubSub.Topic, Subscription: e.PubSub.Subscription}
	}
	if e.Kafka != nil {
		brokers := append([]string(nil), e.Kafka.BootstrapServers...)
		out.Kafka = &KafkaEvent{BootstrapServers: brokers, Topic: e.Kafka.Topic, GroupID: e.Kafka.GroupID}
	}
	if e.RabbitMQ != nil {
		out.RabbitMQ = &RabbitMQEvent{URL: e.RabbitMQ.URL, Queue: e.RabbitMQ.Queue}
	}
	return out
}
