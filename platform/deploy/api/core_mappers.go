package api

import state "github.com/runfabric/runfabric/platform/core/state/core"

func toReceiptRecord(in *state.Receipt) *ReceiptRecord {
	if in == nil {
		return nil
	}
	out := &ReceiptRecord{
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
		out.Artifacts = make([]ReceiptArtifact, 0, len(in.Artifacts))
		for _, a := range in.Artifacts {
			out.Artifacts = append(out.Artifacts, ReceiptArtifact{
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
		out.Functions = make([]ReceiptFunctionDeployment, 0, len(in.Functions))
		for _, f := range in.Functions {
			out.Functions = append(out.Functions, ReceiptFunctionDeployment{
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

func toCoreReceipt(in *ReceiptRecord) *state.Receipt {
	if in == nil {
		return nil
	}
	out := &state.Receipt{
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
		out.Artifacts = make([]state.Artifact, 0, len(in.Artifacts))
		for _, a := range in.Artifacts {
			out.Artifacts = append(out.Artifacts, state.Artifact{
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
		out.Functions = make([]state.FunctionDeployment, 0, len(in.Functions))
		for _, f := range in.Functions {
			out.Functions = append(out.Functions, state.FunctionDeployment{
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
