package lifecycle

// BuildResult is a minimal result for lifecycle build. The main build is app.Build; this exists to avoid lifecycle->app import cycle.
type BuildResult struct {
	Artifacts []interface{} `json:"artifacts"`
}

// Build is the lifecycle-mode build entrypoint. The main build path is app.Build (used by CLI build/package and plan/deploy).
// Lifecycle does not import app to avoid cycles; callers that need real artifacts should use app.Build from the CLI/app layer.
func Build(_ string, _ bool) (*BuildResult, error) {
	return &BuildResult{}, nil
}
