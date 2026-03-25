package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/runfabric/runfabric/platform/core/model/config"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// lambdaEffectiveHandler returns the handler string for Lambda given the zip layout: we package
// the file as handler.js at zip root, so Lambda handler must be "handler.handler" not "dist/handler.handler".
func lambdaEffectiveHandler(handler string) string {
	if handler == "" {
		return "handler.handler"
	}
	parts := strings.SplitN(handler, ".", 2)
	if len(parts) < 2 {
		return handler
	}
	base := filepath.Base(parts[0])
	return base + "." + parts[1]
}

type deployedLambda struct {
	FunctionName  string
	FunctionArn   string
	InvokeArn     string // ARN used for API Gateway integration (alias when strategy is blue-green/canary, else function ARN)
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
	artifact sdkprovider.Artifact,
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
			effectiveHandler := lambdaEffectiveHandler(fn.Handler)
			createIn := &lambda.CreateFunctionInput{
				FunctionName: &name,
				Role:         &roleARN,
				Runtime:      lambdatypes.Runtime(fn.Runtime),
				Handler:      &effectiveHandler,
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
			if err := applyScaling(ctx, clients, name, fn); err != nil {
				return nil, fmt.Errorf("set lambda scaling after create: %w", err)
			}

			out := &deployedLambda{
				FunctionName: name,
				FunctionArn:  arn,
				InvokeArn:    arn,
				Created:      true,
			}
			if cfg.Deploy != nil && (cfg.Deploy.Strategy == "blue-green" || cfg.Deploy.Strategy == "canary") {
				aliasARN, err := publishVersionAndEnsureAlias(ctx, clients, name, cfg)
				if err != nil {
					return nil, fmt.Errorf("blue-green alias: %w", err)
				}
				out.InvokeArn = aliasARN
			}
			return out, nil
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
		// Ensure Lambda handler matches zip layout (handler.js at root => handler.handler)
		effectiveHandler := lambdaEffectiveHandler(fn.Handler)
		_, err = clients.Lambda.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
			FunctionName: &name,
			Handler:      &effectiveHandler,
		})
		if err != nil {
			return nil, fmt.Errorf("update lambda handler after code deploy: %w", err)
		}
		if err := waitUntilFunctionActive(ctx, clients, name); err != nil {
			return nil, fmt.Errorf("wait after handler update: %w", err)
		}
	}

	if changeSet.ConfigChanged {
		effectiveHandler := lambdaEffectiveHandler(fn.Handler)
		updateCfg := &lambda.UpdateFunctionConfigurationInput{
			FunctionName: &name,
			Role:         &roleARN,
			Runtime:      lambdatypes.Runtime(fn.Runtime),
			Handler:      &effectiveHandler,
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
		if err := applyScaling(ctx, clients, name, fn); err != nil {
			return nil, fmt.Errorf("set lambda scaling after update: %w", err)
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

	invokeArn := arn
	if cfg.Deploy != nil && (cfg.Deploy.Strategy == "blue-green" || cfg.Deploy.Strategy == "canary") {
		aliasARN, err := publishVersionAndEnsureAlias(ctx, clients, name, cfg)
		if err != nil {
			return nil, fmt.Errorf("blue-green alias: %w", err)
		}
		invokeArn = aliasARN
	}

	return &deployedLambda{
		FunctionName:  name,
		FunctionArn:   arn,
		InvokeArn:     invokeArn,
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

// applyScaling sets reserved concurrency and/or provisioned concurrency when specified in config.
func applyScaling(ctx context.Context, clients *AWSClients, functionName string, fn config.FunctionConfig) error {
	if fn.ReservedConcurrency > 0 {
		_, err := clients.Lambda.PutFunctionConcurrency(ctx, &lambda.PutFunctionConcurrencyInput{
			FunctionName:                 &functionName,
			ReservedConcurrentExecutions: aws.Int32(int32(fn.ReservedConcurrency)),
		})
		if err != nil {
			return err
		}
	}
	if fn.ProvisionedConcurrency > 0 {
		qualifier := "$LATEST"
		_, err := clients.Lambda.PutProvisionedConcurrencyConfig(ctx, &lambda.PutProvisionedConcurrencyConfigInput{
			FunctionName:                    &functionName,
			Qualifier:                       &qualifier,
			ProvisionedConcurrentExecutions: aws.Int32(int32(fn.ProvisionedConcurrency)),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
