package app

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	coredevstream "github.com/runfabric/runfabric/platform/core/model/devstream"
	"github.com/runfabric/runfabric/platform/observability/diagnostics"
)

type DevStreamReport struct {
	Provider       string   `json:"provider"`
	CapabilityMode string   `json:"capabilityMode"`
	EffectiveMode  string   `json:"effectiveMode"`
	MissingPrereqs []string `json:"missingPrereqs,omitempty"`
	Message        string   `json:"message,omitempty"`
}

func reportFromStatus(status coredevstream.Status) *DevStreamReport {
	return &DevStreamReport{
		Provider:       status.ProviderName,
		CapabilityMode: string(status.CapabilityMode),
		EffectiveMode:  string(status.EffectiveMode),
		MissingPrereqs: append([]string(nil), status.MissingPrereqs...),
		Message:        status.Message,
	}
}

func validateDevStreamTunnelURL(tunnelURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(tunnelURL))
	if err != nil {
		return "", fmt.Errorf("invalid tunnel URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid tunnel URL: scheme must be http or https")
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("invalid tunnel URL: host required")
	}
	if _, err := net.LookupHost(parsed.Hostname()); err != nil {
		return "", fmt.Errorf("tunnel host lookup failed for %q: %w", parsed.Hostname(), err)
	}
	return fmt.Sprintf("URL is valid and host %q resolves", parsed.Hostname()), nil
}

func appendDevStreamChecks(report *diagnostics.HealthReport, provider string, tunnelURL string) error {
	status := coredevstream.EvaluateProvider(provider)
	capabilityMsg := fmt.Sprintf("capability=%s effective=%s: %s", status.CapabilityMode, status.EffectiveMode, status.Message)
	report.Checks = append(report.Checks, diagnostics.CheckResult{
		Name:    "dev-stream-capability",
		OK:      true,
		Backend: provider,
		Message: capabilityMsg,
	})

	if tunnelURL != "" {
		message, err := validateDevStreamTunnelURL(tunnelURL)
		if err != nil {
			report.Checks = append(report.Checks, diagnostics.CheckResult{
				Name:    "dev-stream-tunnel-url",
				OK:      false,
				Backend: provider,
				Message: err.Error(),
			})
			return err
		}
		report.Checks = append(report.Checks, diagnostics.CheckResult{
			Name:    "dev-stream-tunnel-url",
			OK:      true,
			Backend: provider,
			Message: message,
		})
	}

	mutationOK := true
	mutationMsg := "no provider-side mutation required"
	switch status.CapabilityMode {
	case coredevstream.ModeConditionalMutation:
		if len(status.MissingPrereqs) > 0 {
			mutationOK = false
			mutationMsg = fmt.Sprintf("provider-side mutation unavailable; lifecycle-only fallback will be used because %s", strings.Join(status.MissingPrereqs, ", "))
		} else {
			mutationMsg = "provider-side mutation prerequisites are present"
		}
	case coredevstream.ModeRouteRewrite:
		if len(status.MissingPrereqs) > 0 {
			mutationOK = false
			mutationMsg = fmt.Sprintf("provider-side mutation unavailable; lifecycle-only fallback will be used because %s", strings.Join(status.MissingPrereqs, ", "))
		} else {
			mutationMsg = "provider supports route rewrite; deployed provider resources are validated during prepare"
		}
	case coredevstream.ModeLifecycleOnly:
		mutationMsg = "lifecycle-only provider; manual routing is expected"
	}

	report.Checks = append(report.Checks, diagnostics.CheckResult{
		Name:    "dev-stream-provider-mutation",
		OK:      mutationOK,
		Backend: provider,
		Message: mutationMsg,
	})
	return nil
}
