package runtime

type ArtifactRecord struct {
	Function        string `json:"function"`
	Runtime         string `json:"runtime"`
	SourcePath      string `json:"sourcePath"`
	OutputPath      string `json:"outputPath"`
	SHA256          string `json:"sha256"`
	ConfigSignature string `json:"configSignature,omitempty"`
	BuildKey        string `json:"buildKey"`
}

type Manifest struct {
	Version   int              `json:"version"`
	Service   string           `json:"service"`
	Stage     string           `json:"stage"`
	Artifacts []ArtifactRecord `json:"artifacts"`
}
