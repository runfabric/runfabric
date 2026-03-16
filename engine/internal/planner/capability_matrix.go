// Package planner defines the Trigger Capability Matrix: which triggers each
// provider supports. Kept in sync with docs/EXAMPLES_MATRIX.md.
package planner

// Trigger kind identifiers (align with config event types and EXAMPLES_MATRIX).
const (
	TriggerHTTP        = "http"
	TriggerCron        = "cron"
	TriggerQueue       = "queue"
	TriggerStorage     = "storage"
	TriggerEventBridge = "eventbridge"
	TriggerPubSub      = "pubsub"
	TriggerKafka       = "kafka"
	TriggerRabbitMQ    = "rabbitmq"
)

// ProviderCapabilities maps provider name -> set of supported trigger kinds.
// Y in EXAMPLES_MATRIX = true; N = false.
var ProviderCapabilities = map[string]map[string]bool{
	"aws-lambda": {
		TriggerHTTP: true, TriggerCron: true, TriggerQueue: true,
		TriggerStorage: true, TriggerEventBridge: true,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"gcp-functions": {
		TriggerHTTP: true, TriggerCron: true, TriggerQueue: true,
		TriggerStorage: true, TriggerEventBridge: false, TriggerPubSub: true,
		TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"azure-functions": {
		TriggerHTTP: true, TriggerCron: true, TriggerQueue: true,
		TriggerStorage: true, TriggerEventBridge: false, TriggerPubSub: false,
		TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"kubernetes": {
		TriggerHTTP: true, TriggerCron: true,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"cloudflare-workers": {
		TriggerHTTP: true, TriggerCron: true,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"vercel": {
		TriggerHTTP: true, TriggerCron: true,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"netlify": {
		TriggerHTTP: true, TriggerCron: true,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"alibaba-fc": {
		TriggerHTTP: true, TriggerCron: true, TriggerQueue: true,
		TriggerStorage: true, TriggerEventBridge: false, TriggerPubSub: false,
		TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"digitalocean-functions": {
		TriggerHTTP: true, TriggerCron: true,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"fly-machines": {
		TriggerHTTP: true, TriggerCron: false,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
	"ibm-openwhisk": {
		TriggerHTTP: true, TriggerCron: true,
		TriggerQueue: false, TriggerStorage: false, TriggerEventBridge: false,
		TriggerPubSub: false, TriggerKafka: false, TriggerRabbitMQ: false,
	},
}

// SupportedTriggers returns the list of trigger kinds supported by the given provider.
// Provider name is case-sensitive (e.g. "aws-lambda").
func SupportedTriggers(provider string) []string {
	cap, ok := ProviderCapabilities[provider]
	if !ok {
		return nil
	}
	var out []string
	for trigger, supported := range cap {
		if supported {
			out = append(out, trigger)
		}
	}
	return out
}

// SupportsTrigger reports whether the provider supports the given trigger kind.
func SupportsTrigger(provider, trigger string) bool {
	cap, ok := ProviderCapabilities[provider]
	if !ok {
		return false
	}
	return cap[trigger]
}
