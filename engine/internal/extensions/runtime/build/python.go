package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/engine/internal/config"
	"github.com/runfabric/runfabric/engine/internal/extensions/providers"
	"github.com/runfabric/runfabric/engine/internal/extensions/runtime/artifacts"
)

func PackagePythonFunction(
	root,
	functionName string,
	fn config.FunctionConfig,
	configSignature string,
) (*providers.Artifact, error) {
	handlerFile, archiveName, err := resolvePythonHandlerFile(fn.Handler)
	if err != nil {
		return nil, err
	}

	sourcePath := filepath.Join(root, handlerFile)
	outputPath := filepath.Join(root, ".runfabric", "build", functionName+".zip")

	zipResult, err := artifacts.ZipSingleFile(sourcePath, outputPath, archiveName)
	if err != nil {
		return nil, err
	}

	return &providers.Artifact{
		Function:        functionName,
		Runtime:         fn.Runtime,
		SourcePath:      sourcePath,
		OutputPath:      zipResult.OutputPath,
		SHA256:          zipResult.SHA256,
		SizeBytes:       zipResult.SizeBytes,
		ConfigSignature: configSignature,
	}, nil
}

func resolvePythonHandlerFile(handler string) (sourcePath string, archiveName string, err error) {
	if strings.TrimSpace(handler) == "" {
		return "", "", fmt.Errorf("empty handler")
	}

	parts := strings.Split(handler, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid handler %q, expected file.export", handler)
	}

	filePart := parts[0]
	archiveName = filepath.Base(filePart) + ".py"
	return filePart + ".py", archiveName, nil
}
