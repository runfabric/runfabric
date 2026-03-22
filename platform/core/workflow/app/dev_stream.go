package app

import (
	"context"

	coredevstream "github.com/runfabric/runfabric/platform/core/model/devstream"
	alibabaprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/alibaba"
	awsprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/aws"
	azureprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/azure"
	cfprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/cloudflare"
	digitaloceanprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/digitalocean"
	flyprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/fly"
	gcpprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/gcp"
	ibmprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/ibm"
	kubernetesprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/kubernetes"
	netlifyprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/netlify"
	vercelprovider "github.com/runfabric/runfabric/platform/extensions/interfaces/providers/vercel"
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

	switch p {
	case "aws-lambda":
		state, err := awsprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		report.EffectiveMode = string(coredevstream.ModeRouteRewrite)
		report.Message = "full route rewrite configured; provider state will be restored on exit"
		region := ctx.Config.Provider.Region
		return func() {
			_ = state.Restore(context.Background(), region)
		}, report, nil

	case "gcp-functions":
		state, err := gcpprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = string(state.EffectiveMode)
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background(), ctx.Config.Provider.Region)
			}
		}, report, nil

	case "cloudflare-workers":
		state, err := cfprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = string(state.EffectiveMode)
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background(), "")
			}
		}, report, nil

	case "azure-functions":
		state, err := azureprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "digitalocean-functions":
		state, err := digitaloceanprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "fly-machines":
		state, err := flyprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "kubernetes":
		state, err := kubernetesprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "netlify":
		state, err := netlifyprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "vercel":
		state, err := vercelprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "alibaba-fc":
		state, err := alibabaprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	case "ibm-openwhisk":
		state, err := ibmprovider.RedirectToTunnel(context.Background(), ctx.Config, stage, tunnelURL)
		if err != nil {
			return nil, report, err
		}
		if state != nil {
			report.EffectiveMode = state.Mode
			report.MissingPrereqs = append([]string(nil), state.MissingPrereqs...)
			report.Message = state.StatusMessage
		}
		return func() {
			if state != nil {
				_ = state.Restore(context.Background())
			}
		}, report, nil

	default:
		// Other providers: no auto-wire support
		return nil, report, nil
	}
}
