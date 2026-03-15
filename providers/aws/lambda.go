package aws

import (
	"context"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/internal/config"
	"github.com/runfabric/runfabric/internal/providers"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type deployedLambda struct {
	FunctionName  string
	FunctionArn   string
	URL           string
	Created       bool
	UpdatedCode   bool
	UpdatedConfig bool
	Skipped       bool
}

func upsertLambdaFunction(
	ctx context.Context,
	clients *AWSClients,
	cfg *config.Config,
	stage string,
	fnName string,
	fn config.FunctionConfig,
	artifact providers.Artifact,
	roleARN string,
	changeSet functionChangeSet,
) (*deployedLambda, error) {
	name := functionName(cfg, stage, fnName)

	zipBytes, err := os.ReadFile(artifact.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("read zip artifact: %w", err)
	}

	envVars := environmentMap(fn)
	architectures := lambdaArchitectures(fn)
	layers := lambdaLayers(fn)

	_, err = clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
	})
	if err != nil {
		if isLambdaNotFound(err) {
			createIn := &lambda.CreateFunctionInput{
				FunctionName: &name,
				Role:         &roleARN,
				Runtime:      lambdatypes.Runtime(fn.Runtime),
				Handler:      &fn.Handler,
				Code: &lambdatypes.FunctionCode{
					ZipFile: zipBytes,
				},
				Timeout:    aws.Int32(int32(fn.Timeout)),
				MemorySize: aws.Int32(int32(fn.Memory)),
				Publish:    true,
			}

			if len(envVars) > 0 {
				createIn.Environment = &lambdatypes.Environment{
					Variables: envVars,
				}
			}
			if len(architectures) > 0 {
				createIn.Architectures = architectures
			}
			if len(layers) > 0 {
				createIn.Layers = layers
			}

			createOut, createErr := clients.Lambda.CreateFunction(ctx, createIn)
			if createErr != nil {
				return nil, fmt.Errorf("create lambda: %w", createErr)
			}

			if err := waitUntilFunctionActive(ctx, clients, name); err != nil {
				return nil, fmt.Errorf("wait for created lambda active: %w", err)
			}

			arn := ""
			if createOut.FunctionArn != nil {
				arn = *createOut.FunctionArn
			}

			if err := ensureLambdaTags(ctx, clients, arn, fn.Tags); err != nil {
				return nil, fmt.Errorf("tag lambda after create: %w", err)
			}

			return &deployedLambda{
				FunctionName: name,
				FunctionArn:  arn,
				Created:      true,
			}, nil
		}

		return nil, fmt.Errorf("get lambda: %w", err)
	}

	updatedCode := false
	updatedConfig := false

	if changeSet.CodeChanged {
		_, err = clients.Lambda.UpdateFunctionCode(ctx, &lambda.UpdateFunctionCodeInput{
			FunctionName: &name,
			ZipFile:      zipBytes,
			Publish:      true,
		})
		if err != nil {
			return nil, fmt.Errorf("update lambda code: %w", err)
		}

		if err := waitUntilFunctionActive(ctx, clients, name); err != nil {
			return nil, fmt.Errorf("wait after code update: %w", err)
		}
		updatedCode = true
	}

	if changeSet.ConfigChanged {
		updateCfg := &lambda.UpdateFunctionConfigurationInput{
			FunctionName: &name,
			Role:         &roleARN,
			Runtime:      lambdatypes.Runtime(fn.Runtime),
			Handler:      &fn.Handler,
			Timeout:      aws.Int32(int32(fn.Timeout)),
			MemorySize:   aws.Int32(int32(fn.Memory)),
		}

		if len(envVars) > 0 {
			updateCfg.Environment = &lambdatypes.Environment{
				Variables: envVars,
			}
		}

		if len(layers) > 0 {
			updateCfg.Layers = layers
		}

		_, err = clients.Lambda.UpdateFunctionConfiguration(ctx, updateCfg)
		if err != nil {
			return nil, fmt.Errorf("update lambda config: %w", err)
		}

		if err := waitUntilFunctionActive(ctx, clients, name); err != nil {
			return nil, fmt.Errorf("wait after config update: %w", err)
		}
		updatedConfig = true
	}

	getOut, err := clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
	})
	if err != nil {
		return nil, fmt.Errorf("re-read lambda: %w", err)
	}

	arn := ""
	if getOut.Configuration != nil && getOut.Configuration.FunctionArn != nil {
		arn = *getOut.Configuration.FunctionArn
	}

	if err := ensureLambdaTags(ctx, clients, arn, fn.Tags); err != nil {
		return nil, fmt.Errorf("tag lambda after update: %w", err)
	}

	return &deployedLambda{
		FunctionName:  name,
		FunctionArn:   arn,
		UpdatedCode:   updatedCode,
		UpdatedConfig: updatedConfig,
		Skipped:       !updatedCode && !updatedConfig,
	}, nil
}

func lambdaArchitectures(fn config.FunctionConfig) []lambdatypes.Architecture {
	switch fn.Architecture {
	case "x86_64":
		return []lambdatypes.Architecture{lambdatypes.ArchitectureX8664}
	case "arm64":
		return []lambdatypes.Architecture{lambdatypes.ArchitectureArm64}
	default:
		return nil
	}
}

func lambdaLayers(fn config.FunctionConfig) []string {
	if len(fn.Layers) == 0 {
		return nil
	}
	return fn.Layers
}
