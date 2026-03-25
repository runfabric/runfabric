package aws

import (
	"context"

	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Deploy(ctx context.Context, req providers.DeployRequest) (*providers.DeployResult, error) {
	return nil, appErrs.Wrap(
		appErrs.CodeDeployFailed,
		"direct provider deploy is no longer supported; use the control-plane deploy path",
		nil,
	)
}

func artifactsFromMap(m map[string]sdkprovider.Artifact) []sdkprovider.Artifact {
	out := make([]sdkprovider.Artifact, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out
}

func stateArtifactsFromProvider(in []sdkprovider.Artifact) []state.Artifact {
	out := make([]state.Artifact, 0, len(in))
	for _, a := range in {
		out = append(out, state.Artifact{
			Function:        a.Function,
			Runtime:         a.Runtime,
			SourcePath:      a.SourcePath,
			OutputPath:      a.OutputPath,
			SHA256:          a.SHA256,
			SizeBytes:       a.SizeBytes,
			ConfigSignature: a.ConfigSignature,
		})
	}
	return out
}

func sdkArtifactFromCore(a providers.Artifact) sdkprovider.Artifact {
	return sdkprovider.Artifact{
		Function:        a.Function,
		Runtime:         a.Runtime,
		SourcePath:      a.SourcePath,
		OutputPath:      a.OutputPath,
		SHA256:          a.SHA256,
		SizeBytes:       a.SizeBytes,
		ConfigSignature: a.ConfigSignature,
	}
}
