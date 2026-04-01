package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/deploy/source"
)

// DeployFromSourceURL fetches and extracts an archive source, then deploys using app.Deploy.
// If configPath points to a non-default file, that config is copied into the extracted root.
func DeployFromSourceURL(configPath, sourceURL, stage, functionName string, rollbackOnFailure, noRollbackOnFailure bool, providerOverride string) (any, error) {
	extractRoot, resolvedConfig, cleanup, err := source.FetchAndExtract(sourceURL)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if isCustomConfigPath(configPath) {
		destConfig := filepath.Join(extractRoot, "runfabric.yml")
		if err := copyConfigFile(configPath, destConfig); err != nil {
			return nil, err
		}
		resolvedConfig = destConfig
	}

	return Deploy(
		resolvedConfig,
		stage,
		functionName,
		rollbackOnFailure,
		noRollbackOnFailure,
		nil,
		providerOverride,
	)
}

func isCustomConfigPath(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}
	switch p {
	case "runfabric.yml", "runfabric.yaml":
		return false
	default:
		return true
	}
}

func copyConfigFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
