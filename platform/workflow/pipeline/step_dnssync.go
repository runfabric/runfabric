package pipeline

import (
	"context"
	"io"
	"strings"
)

// DNSSyncFunc is the function signature for running a router DNS sync.
// Injected so the pipeline package does not import platform/workflow/app (circular).
type DNSSyncFunc func(ctx context.Context, hostname, serviceURL, service, stage string, out io.Writer) error

// DNSSyncStep automatically syncs DNS after deploy when extensions.router.hostname
// is configured and autoApply is enabled. It is a no-op otherwise.
type DNSSyncStep struct {
	// AutoApply gates the step — set from RouterDNSSyncPolicy.AutoApply.
	AutoApply bool
	// Sync is the injected DNS sync function.
	Sync DNSSyncFunc
	// Out receives sync progress output.
	Out io.Writer
}

func (s DNSSyncStep) Name() string { return "dns-sync" }

func (s DNSSyncStep) Run(ctx context.Context, sc *StepContext) error {
	if !s.AutoApply || s.Sync == nil {
		return nil
	}

	// Hostname comes from extensions.router.hostname.
	hostname := routerHostname(sc)
	if hostname == "" {
		return nil
	}

	// Endpoint URL comes from deploy result.
	url := ""
	if sc.DeployResult != nil {
		for _, k := range []string{"url", "ServiceURL", "ApiUrl", "endpoint"} {
			if v := strings.TrimSpace(sc.DeployResult.Outputs[k]); v != "" {
				url = v
				break
			}
		}
	}
	if url == "" {
		return nil
	}

	out := s.Out
	if out == nil {
		out = io.Discard
	}

	return s.Sync(ctx, hostname, url, sc.Config.Service, sc.Stage, out)
}

// routerHostname reads extensions.router.hostname from the step config.
func routerHostname(sc *StepContext) string {
	if sc.Config == nil || sc.Config.Extensions == nil {
		return ""
	}
	router, ok := sc.Config.Extensions["router"].(map[string]any)
	if !ok {
		return ""
	}
	h, _ := router["hostname"].(string)
	return strings.TrimSpace(h)
}
