package receiptconv

import (
	statetypes "github.com/runfabric/runfabric/internal/state/types"
	corestate "github.com/runfabric/runfabric/platform/core/state/core"
)

func ToCoreReceipt(in *statetypes.Receipt) *corestate.Receipt {
	if in == nil {
		return nil
	}
	out := &corestate.Receipt{
		Version:      in.Version,
		Service:      in.Service,
		Stage:        in.Stage,
		Provider:     in.Provider,
		DeploymentID: in.DeploymentID,
		Outputs:      cloneStringMap(in.Outputs),
		Metadata:     cloneStringMap(in.Metadata),
		UpdatedAt:    in.UpdatedAt,
	}
	if len(in.Artifacts) > 0 {
		out.Artifacts = make([]corestate.Artifact, 0, len(in.Artifacts))
		for _, a := range in.Artifacts {
			out.Artifacts = append(out.Artifacts, corestate.Artifact{
				Function:        a.Function,
				Runtime:         a.Runtime,
				SourcePath:      a.SourcePath,
				OutputPath:      a.OutputPath,
				SHA256:          a.SHA256,
				SizeBytes:       a.SizeBytes,
				ConfigSignature: a.ConfigSignature,
			})
		}
	}
	if len(in.Functions) > 0 {
		out.Functions = make([]corestate.FunctionDeployment, 0, len(in.Functions))
		for _, f := range in.Functions {
			out.Functions = append(out.Functions, corestate.FunctionDeployment{
				Function:           f.Function,
				ArtifactSHA256:     f.ArtifactSHA256,
				ConfigSignature:    f.ConfigSignature,
				ResourceName:       f.ResourceName,
				ResourceIdentifier: f.ResourceIdentifier,
				Metadata:           cloneStringMap(f.Metadata),
				EnvironmentHash:    f.EnvironmentHash,
				TagsHash:           f.TagsHash,
				LayersHash:         f.LayersHash,
			})
		}
	}
	return out
}

func FromCoreReceipt(in *corestate.Receipt) *statetypes.Receipt {
	if in == nil {
		return nil
	}
	out := &statetypes.Receipt{
		Version:      in.Version,
		Service:      in.Service,
		Stage:        in.Stage,
		Provider:     in.Provider,
		DeploymentID: in.DeploymentID,
		Outputs:      cloneStringMap(in.Outputs),
		Metadata:     cloneStringMap(in.Metadata),
		UpdatedAt:    in.UpdatedAt,
	}
	if len(in.Artifacts) > 0 {
		out.Artifacts = make([]statetypes.Artifact, 0, len(in.Artifacts))
		for _, a := range in.Artifacts {
			out.Artifacts = append(out.Artifacts, statetypes.Artifact{
				Function:        a.Function,
				Runtime:         a.Runtime,
				SourcePath:      a.SourcePath,
				OutputPath:      a.OutputPath,
				SHA256:          a.SHA256,
				SizeBytes:       a.SizeBytes,
				ConfigSignature: a.ConfigSignature,
			})
		}
	}
	if len(in.Functions) > 0 {
		out.Functions = make([]statetypes.FunctionDeployment, 0, len(in.Functions))
		for _, f := range in.Functions {
			out.Functions = append(out.Functions, statetypes.FunctionDeployment{
				Function:           f.Function,
				ArtifactSHA256:     f.ArtifactSHA256,
				ConfigSignature:    f.ConfigSignature,
				ResourceName:       f.ResourceName,
				ResourceIdentifier: f.ResourceIdentifier,
				Metadata:           cloneStringMap(f.Metadata),
				EnvironmentHash:    f.EnvironmentHash,
				TagsHash:           f.TagsHash,
				LayersHash:         f.LayersHash,
			})
		}
	}
	return out
}

func FromCoreReleaseEntries(in []corestate.ReleaseEntry) []statetypes.ReleaseEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]statetypes.ReleaseEntry, 0, len(in))
	for _, item := range in {
		out = append(out, statetypes.ReleaseEntry{
			Stage:     item.Stage,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
