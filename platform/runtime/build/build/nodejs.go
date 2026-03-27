package build

import (
	"fmt"
	"path/filepath"
	"strings"

	extproviders "github.com/runfabric/runfabric/internal/provider/contracts"
	"github.com/runfabric/runfabric/platform/core/model/config"
	"github.com/runfabric/runfabric/platform/runtime/build/artifacts"
)

func PackageNodeFunction(
	root,
	functionName string,
	fn config.FunctionConfig,
	configSignature string,
) (*extproviders.Artifact, error) {
	handlerFile, archiveName, err := resolveHandlerFile(fn.Handler)
	if err != nil {
		return nil, err
	}

	sourcePath := filepath.Join(root, handlerFile)
	outputPath := filepath.Join(root, ".runfabric", "build", functionName+".zip")

	zipResult, err := artifacts.ZipSingleFile(sourcePath, outputPath, archiveName)
	if err != nil {
		return nil, err
	}

	return &extproviders.Artifact{
		Function:        functionName,
		Runtime:         fn.Runtime,
		SourcePath:      sourcePath,
		OutputPath:      zipResult.OutputPath,
		SHA256:          zipResult.SHA256,
		SizeBytes:       zipResult.SizeBytes,
		ConfigSignature: configSignature,
	}, nil
}

func resolveHandlerFile(handler string) (sourcePath string, archiveName string, err error) {
	if strings.TrimSpace(handler) == "" {
		return "", "", fmt.Errorf("empty handler")
	}

	parts := strings.Split(handler, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid handler %q, expected file.export", handler)
	}

	filePart := parts[0]
	// Put handler.js at zip root so Lambda handler "handler.handler" works (AWS provider sets effective handler).
	archiveName = filepath.Base(filePart) + ".js"
	return filePart + ".js", archiveName, nil
}
