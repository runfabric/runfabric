package app

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/provider"
	coredevstream "github.com/runfabric/runfabric/platform/core/model/devstream"
)

// PrepareDevStreamTunnel redirects the provider's invocation target (e.g. API Gateway) to tunnelURL
// for the given stage. Returns a restore function to call on exit to revert the change.
// AWS performs a real route rewrite. Other built-in API providers return a provider-specific
// lifecycle hook handle so the CLI can run consistent prepare/restore flow without implying
// full route rewrite parity where the platform does not support it.
func PrepareDevStreamTunnel(configPath, stage, tunnelURL string) (restore func(), err error) {
	restore, _, err = PrepareDevStreamTunnelWithReport(configPath, stage, tunnelURL)
	return restore, err
}

func PrepareDevStreamTunnelWithReport(configPath, stage, tunnelURL string) (restore func(), report *DevStreamReport, err error) {
	if tunnelURL == "" {
		return nil, nil, nil
	}
	ctx, err := Bootstrap(configPath, stage, "")
	if err != nil {
		return nil, nil, err
	}
	p := ctx.Config.Provider.Name
	status := coredevstream.EvaluateProvider(p)
	report = reportFromStatus(status)
	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, report, nil
	}
	p = provider.name
	status = coredevstream.EvaluateProvider(p)
	report = reportFromStatus(status)
	devstreamCapable, ok := provider.provider.(providers.DevStreamCapable)
	if !ok {
		return nil, report, nil
	}
	session, err := devstreamCapable.PrepareDevStream(context.Background(), providers.DevStreamRequest{
		Config:    ctx.Config,
		Stage:     stage,
		TunnelURL: tunnelURL,
		Region:    ctx.Config.Provider.Region,
	})
	if err != nil {
		return nil, report, err
	}
	if session == nil {
		return nil, report, nil
	}
	if session.EffectiveMode != "" {
		report.EffectiveMode = session.EffectiveMode
	}
	if len(session.MissingPrereqs) > 0 {
		report.MissingPrereqs = append([]string(nil), session.MissingPrereqs...)
	}
	if session.StatusMessage != "" {
		report.Message = session.StatusMessage
	}
	return func() {
		_ = session.Restore(context.Background())
	}, report, nil
}
