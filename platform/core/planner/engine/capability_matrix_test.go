package engine

import (
	"testing"
)

func TestSupportsTrigger(t *testing.T) {
	tests := []struct {
		provider string
		trigger  string
		want     bool
	}{
		{"aws-lambda", TriggerHTTP, true},
		{"aws-lambda", TriggerEventBridge, true},
		{"aws-lambda", TriggerPubSub, false},
		{"gcp-functions", TriggerPubSub, true},
		{"gcp-functions", TriggerEventBridge, false},
		{"fly-machines", TriggerHTTP, true},
		{"fly-machines", TriggerCron, false},
		{"kubernetes", TriggerQueue, false},
		{"unknown-provider", TriggerHTTP, false},
	}
	for _, tt := range tests {
		got := SupportsTrigger(tt.provider, tt.trigger)
		if got != tt.want {
			t.Errorf("SupportsTrigger(%q, %q) = %v, want %v", tt.provider, tt.trigger, got, tt.want)
		}
	}
}

func TestSupportedTriggers(t *testing.T) {
	got := SupportedTriggers("aws-lambda")
	if len(got) == 0 {
		t.Error("SupportedTriggers(aws-lambda) returned empty")
	}
	found := false
	for _, trig := range got {
		if trig == TriggerHTTP {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SupportedTriggers(aws-lambda) should include %q, got %v", TriggerHTTP, got)
	}

	gotUnknown := SupportedTriggers("unknown-provider")
	if gotUnknown != nil {
		t.Errorf("SupportedTriggers(unknown-provider) want nil, got %v", gotUnknown)
	}
}

// Providers from docs/docs/EXAMPLES_MATRIX.md; keep in sync with ProviderCapabilities.
var expectedMatrixProviders = []string{
	"aws-lambda", "gcp-functions", "azure-functions", "kubernetes",
	"cloudflare-workers", "vercel", "netlify", "alibaba-fc",
	"digitalocean-functions", "fly-machines", "ibm-openwhisk",
}

func TestCapabilityMatrixSyncWithDocs(t *testing.T) {
	for _, name := range expectedMatrixProviders {
		if _, ok := ProviderCapabilities[name]; !ok {
			t.Errorf("ProviderCapabilities missing provider %q (add to match EXAMPLES_MATRIX.md)", name)
		}
	}
	for name := range ProviderCapabilities {
		found := false
		for _, expected := range expectedMatrixProviders {
			if expected == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ProviderCapabilities has %q but not listed in expectedMatrixProviders (sync with EXAMPLES_MATRIX.md)", name)
		}
	}
}

// docMatrix encodes the Trigger Capability Matrix from docs/docs/EXAMPLES_MATRIX.md.
// Each key is "provider|trigger"; value true = Y, false = N. Used to assert Go code matches docs.
var docMatrix = map[string]bool{
	"aws-lambda|http": true, "aws-lambda|cron": true, "aws-lambda|queue": true, "aws-lambda|storage": true,
	"aws-lambda|eventbridge": true, "aws-lambda|pubsub": false, "aws-lambda|kafka": false, "aws-lambda|rabbitmq": false,
	"gcp-functions|http": true, "gcp-functions|cron": true, "gcp-functions|queue": true, "gcp-functions|storage": true,
	"gcp-functions|eventbridge": false, "gcp-functions|pubsub": true, "gcp-functions|kafka": false, "gcp-functions|rabbitmq": false,
	"azure-functions|http": true, "azure-functions|cron": true, "azure-functions|queue": true, "azure-functions|storage": true,
	"azure-functions|eventbridge": false, "azure-functions|pubsub": false, "azure-functions|kafka": false, "azure-functions|rabbitmq": false,
	"kubernetes|http": true, "kubernetes|cron": true, "kubernetes|queue": false, "kubernetes|storage": false,
	"kubernetes|eventbridge": false, "kubernetes|pubsub": false, "kubernetes|kafka": false, "kubernetes|rabbitmq": false,
	"cloudflare-workers|http": true, "cloudflare-workers|cron": true, "cloudflare-workers|queue": false, "cloudflare-workers|storage": false,
	"cloudflare-workers|eventbridge": false, "cloudflare-workers|pubsub": false, "cloudflare-workers|kafka": false, "cloudflare-workers|rabbitmq": false,
	"vercel|http": true, "vercel|cron": true, "vercel|queue": false, "vercel|storage": false,
	"vercel|eventbridge": false, "vercel|pubsub": false, "vercel|kafka": false, "vercel|rabbitmq": false,
	"netlify|http": true, "netlify|cron": true, "netlify|queue": false, "netlify|storage": false,
	"netlify|eventbridge": false, "netlify|pubsub": false, "netlify|kafka": false, "netlify|rabbitmq": false,
	"alibaba-fc|http": true, "alibaba-fc|cron": true, "alibaba-fc|queue": true, "alibaba-fc|storage": true,
	"alibaba-fc|eventbridge": false, "alibaba-fc|pubsub": false, "alibaba-fc|kafka": false, "alibaba-fc|rabbitmq": false,
	"digitalocean-functions|http": true, "digitalocean-functions|cron": true, "digitalocean-functions|queue": false, "digitalocean-functions|storage": false,
	"digitalocean-functions|eventbridge": false, "digitalocean-functions|pubsub": false, "digitalocean-functions|kafka": false, "digitalocean-functions|rabbitmq": false,
	"fly-machines|http": true, "fly-machines|cron": false, "fly-machines|queue": false, "fly-machines|storage": false,
	"fly-machines|eventbridge": false, "fly-machines|pubsub": false, "fly-machines|kafka": false, "fly-machines|rabbitmq": false,
	"ibm-openwhisk|http": true, "ibm-openwhisk|cron": true, "ibm-openwhisk|queue": false, "ibm-openwhisk|storage": false,
	"ibm-openwhisk|eventbridge": false, "ibm-openwhisk|pubsub": false, "ibm-openwhisk|kafka": false, "ibm-openwhisk|rabbitmq": false,
}

var docTriggers = []string{TriggerHTTP, TriggerCron, TriggerQueue, TriggerStorage, TriggerEventBridge, TriggerPubSub, TriggerKafka, TriggerRabbitMQ}

// TestTriggerCapabilityMatrixMatchesDocs asserts ProviderCapabilities matches docs/docs/EXAMPLES_MATRIX.md.
func TestTriggerCapabilityMatrixMatchesDocs(t *testing.T) {
	for _, provider := range expectedMatrixProviders {
		for _, trigger := range docTriggers {
			key := provider + "|" + trigger
			want, ok := docMatrix[key]
			if !ok {
				t.Errorf("docMatrix missing %q (add to test from EXAMPLES_MATRIX.md)", key)
				continue
			}
			got := SupportsTrigger(provider, trigger)
			if got != want {
				t.Errorf("SupportsTrigger(%q, %q) = %v, doc says %v (EXAMPLES_MATRIX.md)", provider, trigger, got, want)
			}
		}
	}
}
