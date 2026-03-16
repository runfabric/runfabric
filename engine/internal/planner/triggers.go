// Package planner provides trigger extraction from config and validation against
// the Trigger Capability Matrix so all providers can use a single source of truth.

package planner

import (
	"fmt"

	"github.com/runfabric/runfabric/engine/internal/config"
)

// TriggerSpec describes one trigger bound to a function (kind + optional config).
type TriggerSpec struct {
	Kind   string         // TriggerHTTP, TriggerCron, etc.
	Config map[string]any // trigger-specific params (queue name, cron expr, etc.)
}

// FunctionTriggers holds the list of trigger specs for one function.
type FunctionTriggers struct {
	Function string
	Specs    []TriggerSpec
}

// TriggerKindFromEvent returns the trigger kind for an event config, or empty if none set.
func TriggerKindFromEvent(ev config.EventConfig) string {
	if ev.HTTP != nil {
		return TriggerHTTP
	}
	if ev.Cron != "" {
		return TriggerCron
	}
	if ev.Queue != nil {
		return TriggerQueue
	}
	if ev.Storage != nil {
		return TriggerStorage
	}
	if ev.EventBridge != nil {
		return TriggerEventBridge
	}
	if ev.PubSub != nil {
		return TriggerPubSub
	}
	if ev.Kafka != nil {
		return TriggerKafka
	}
	if ev.RabbitMQ != nil {
		return TriggerRabbitMQ
	}
	return ""
}

// ExtractTriggers returns trigger specs for all functions in the config.
func ExtractTriggers(cfg *config.Config) []FunctionTriggers {
	var out []FunctionTriggers
	for fnName, fn := range cfg.Functions {
		var specs []TriggerSpec
		for _, ev := range fn.Events {
			kind := TriggerKindFromEvent(ev)
			if kind == "" {
				continue
			}
			spec := TriggerSpec{Kind: kind, Config: make(map[string]any)}
			switch kind {
			case TriggerHTTP:
				if ev.HTTP != nil {
					spec.Config["path"] = ev.HTTP.Path
					spec.Config["method"] = ev.HTTP.Method
				}
			case TriggerCron:
				spec.Config["expression"] = ev.Cron
			case TriggerQueue:
				if ev.Queue != nil {
					spec.Config["queue"] = ev.Queue.Queue
					spec.Config["batchSize"] = ev.Queue.Batch
				}
			case TriggerStorage:
				if ev.Storage != nil {
					spec.Config["bucket"] = ev.Storage.Bucket
					spec.Config["prefix"] = ev.Storage.Prefix
					spec.Config["events"] = ev.Storage.Events
				}
			case TriggerEventBridge:
				if ev.EventBridge != nil {
					spec.Config["pattern"] = ev.EventBridge.Pattern
					spec.Config["bus"] = ev.EventBridge.Bus
				}
			case TriggerPubSub:
				if ev.PubSub != nil {
					spec.Config["topic"] = ev.PubSub.Topic
					spec.Config["subscription"] = ev.PubSub.Subscription
				}
			case TriggerKafka:
				if ev.Kafka != nil {
					spec.Config["bootstrapServers"] = ev.Kafka.BootstrapServers
					spec.Config["topic"] = ev.Kafka.Topic
					spec.Config["groupId"] = ev.Kafka.GroupID
				}
			case TriggerRabbitMQ:
				if ev.RabbitMQ != nil {
					spec.Config["url"] = ev.RabbitMQ.URL
					spec.Config["queue"] = ev.RabbitMQ.Queue
				}
			}
			specs = append(specs, spec)
		}
		if len(specs) > 0 {
			out = append(out, FunctionTriggers{Function: fnName, Specs: specs})
		}
	}
	return out
}

// ValidateTriggersForProvider returns errors for any trigger not supported by the provider per capability matrix.
func ValidateTriggersForProvider(cfg *config.Config, provider string) []string {
	var errs []string
	for fnName, fn := range cfg.Functions {
		for _, ev := range fn.Events {
			kind := TriggerKindFromEvent(ev)
			if kind == "" {
				continue
			}
			if !SupportsTrigger(provider, kind) {
				errs = append(errs, fmt.Sprintf("functions.%s: trigger %q is not supported by provider %q (see Trigger Capability Matrix)", fnName, kind, provider))
			}
		}
	}
	return errs
}

// ResourceTypeForTrigger returns the planner ResourceType for a trigger kind.
func ResourceTypeForTrigger(kind string) ResourceType {
	switch kind {
	case TriggerHTTP:
		return ResourceHTTPAPI
	case TriggerCron:
		return ResourceSchedule
	case TriggerQueue:
		return ResourceQueue
	case TriggerStorage:
		return ResourceStorage
	case TriggerEventBridge:
		return ResourceEventBridge
	case TriggerPubSub:
		return ResourcePubSub
	case TriggerKafka:
		return ResourceKafka
	case TriggerRabbitMQ:
		return ResourceRabbitMQ
	default:
		return ResourceLambda
	}
}
