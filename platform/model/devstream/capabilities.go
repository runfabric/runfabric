package devstream

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Mode string

const (
	ModeLifecycleOnly       Mode = "lifecycle-only"
	ModeConditionalMutation Mode = "conditional-mutation"
	ModeRouteRewrite        Mode = "route-rewrite"
)

// Capability describes the coded dev-stream contract for a provider.
type Capability struct {
	ProviderName          string
	CapabilityMode        Mode
	Summary               string
	ManualRoutingRequired bool
}

// Status describes the effective dev-stream mode for the current environment.
type Status struct {
	ProviderName          string
	CapabilityMode        Mode
	EffectiveMode         Mode
	MissingPrereqs        []string
	Message               string
	ManualRoutingRequired bool
}

func CapabilityForProvider(providerName string) Capability {
	switch strings.TrimSpace(providerName) {
	case "aws-lambda":
		return Capability{
			ProviderName:   providerName,
			CapabilityMode: ModeRouteRewrite,
			Summary:        "full route rewrite when a deployed HTTP API exists for the selected stage",
		}
	case "cloudflare-workers":
		return Capability{
			ProviderName:   providerName,
			CapabilityMode: ModeRouteRewrite,
			Summary:        "full route rewrite by temporarily repointing Workers routes to a tunnel proxy worker and restoring on exit",
		}
	case "gcp-functions":
		return Capability{
			ProviderName:          providerName,
			CapabilityMode:        ModeConditionalMutation,
			Summary:               "lifecycle hook always available; provider-side mutation is attempted when GCP prerequisites or gateway rewrite hooks are present",
			ManualRoutingRequired: true,
		}
	case "azure-functions", "digitalocean-functions", "fly-machines", "kubernetes", "netlify", "vercel", "alibaba-fc", "ibm-openwhisk":
		return Capability{
			ProviderName:          providerName,
			CapabilityMode:        ModeConditionalMutation,
			Summary:               "lifecycle hook always available; optional gateway rewrite hooks can apply reversible route rewrite",
			ManualRoutingRequired: true,
		}
	default:
		return Capability{ProviderName: providerName, CapabilityMode: ModeLifecycleOnly, Summary: "no built-in auto-wire contract", ManualRoutingRequired: true}
	}
}

func EvaluateProvider(providerName string) Status {
	capability := CapabilityForProvider(providerName)
	status := Status{
		ProviderName:          capability.ProviderName,
		CapabilityMode:        capability.CapabilityMode,
		EffectiveMode:         capability.CapabilityMode,
		ManualRoutingRequired: capability.ManualRoutingRequired,
	}

	switch capability.ProviderName {
	case "gcp-functions":
		setURL, restoreURL := gatewayHookEnvKeys(capability.ProviderName)
		if strings.TrimSpace(os.Getenv(setURL)) != "" && strings.TrimSpace(os.Getenv(restoreURL)) != "" {
			status.EffectiveMode = ModeRouteRewrite
			status.ManualRoutingRequired = false
			status.Message = "gateway rewrite hooks are configured; prepare step will apply reversible route rewrite through configured gateway"
			return status
		}
		if v := strings.TrimSpace(os.Getenv("GCP_ACCESS_TOKEN")); v == "" {
			status.MissingPrereqs = append(status.MissingPrereqs, "GCP_ACCESS_TOKEN")
		}
		project := strings.TrimSpace(os.Getenv("GCP_PROJECT"))
		projectID := strings.TrimSpace(os.Getenv("GCP_PROJECT_ID"))
		if project == "" && projectID == "" {
			status.MissingPrereqs = append(status.MissingPrereqs, "GCP_PROJECT or GCP_PROJECT_ID")
		}
	case "cloudflare-workers":
		if v := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")); v == "" {
			status.MissingPrereqs = append(status.MissingPrereqs, "CLOUDFLARE_API_TOKEN")
		}
		if v := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")); v == "" {
			status.MissingPrereqs = append(status.MissingPrereqs, "CLOUDFLARE_ACCOUNT_ID")
		}
		if v := strings.TrimSpace(os.Getenv("CLOUDFLARE_ZONE_ID")); v == "" {
			status.MissingPrereqs = append(status.MissingPrereqs, "CLOUDFLARE_ZONE_ID")
		}
	case "azure-functions", "digitalocean-functions", "fly-machines", "kubernetes", "netlify", "vercel", "alibaba-fc", "ibm-openwhisk":
		setURL, restoreURL := gatewayHookEnvKeys(capability.ProviderName)
		setConfigured := strings.TrimSpace(os.Getenv(setURL)) != ""
		restoreConfigured := strings.TrimSpace(os.Getenv(restoreURL)) != ""
		if !setConfigured {
			status.MissingPrereqs = append(status.MissingPrereqs, setURL)
		}
		if !restoreConfigured {
			status.MissingPrereqs = append(status.MissingPrereqs, restoreURL)
		}
		if setConfigured && restoreConfigured {
			status.EffectiveMode = ModeRouteRewrite
			status.ManualRoutingRequired = false
			status.Message = "gateway rewrite hooks are configured; prepare step will apply reversible route rewrite through configured gateway"
			return status
		}
	}

	if len(status.MissingPrereqs) > 0 {
		status.EffectiveMode = ModeLifecycleOnly
		status.Message = fmt.Sprintf("provider-side mutation unavailable; falling back to lifecycle-only because %s", strings.Join(status.MissingPrereqs, ", "))
		return status
	}

	switch capability.CapabilityMode {
	case ModeRouteRewrite:
		status.Message = "provider supports full route rewrite; prepare step will validate deployed provider resources"
	case ModeConditionalMutation:
		status.Message = "provider-side mutation prerequisites are present; prepare step will attempt cloud mutation and fall back only if the provider API rejects the update"
	default:
		status.Message = "lifecycle-only mode; manual provider routing is still required"
	}
	return status
}

func gatewayHookEnvKeys(providerName string) (setURL string, restoreURL string) {
	prefix := gatewayHookEnvPrefix(providerName)
	return "RUNFABRIC_DEV_STREAM_" + prefix + "_SET_URL", "RUNFABRIC_DEV_STREAM_" + prefix + "_RESTORE_URL"
}

func gatewayHookEnvPrefix(providerName string) string {
	upper := strings.ToUpper(strings.TrimSpace(providerName))
	if upper == "" {
		return "PROVIDER"
	}
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	prefix := re.ReplaceAllString(upper, "_")
	prefix = strings.Trim(prefix, "_")
	if prefix == "" {
		return "PROVIDER"
	}
	return prefix
}
