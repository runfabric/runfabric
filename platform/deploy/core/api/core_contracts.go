package api

// ReceiptFunctionDeployment is the deploy-core boundary DTO for per-function deployment metadata.
type ReceiptFunctionDeployment struct {
	Function           string            `json:"function"`
	ArtifactSHA256     string            `json:"artifactSha256"`
	ConfigSignature    string            `json:"configSignature"`
	ResourceName       string            `json:"resourceName,omitempty"`
	ResourceIdentifier string            `json:"resourceIdentifier,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	EnvironmentHash    string            `json:"environmentHash,omitempty"`
	TagsHash           string            `json:"tagsHash,omitempty"`
	LayersHash         string            `json:"layersHash,omitempty"`
}

// ReceiptArtifact is the deploy-core boundary DTO for built artifacts.
type ReceiptArtifact struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath"`
	OutputPath      string `json:"outputPath"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"sizeBytes"`
	ConfigSignature string `json:"configSignature,omitempty"`
}

// ReceiptRecord is the deploy-core boundary DTO for persisted deployment receipts.
type ReceiptRecord struct {
	Version      int                         `json:"version"`
	Service      string                      `json:"service"`
	Stage        string                      `json:"stage"`
	Provider     string                      `json:"provider"`
	DeploymentID string                      `json:"deploymentId"`
	Outputs      map[string]string           `json:"outputs"`
	Artifacts    []ReceiptArtifact           `json:"artifacts,omitempty"`
	Metadata     map[string]string           `json:"metadata,omitempty"`
	Functions    []ReceiptFunctionDeployment `json:"functions,omitempty"`
	UpdatedAt    string                      `json:"updatedAt"`
}
