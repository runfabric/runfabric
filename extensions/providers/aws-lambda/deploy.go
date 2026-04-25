// Package aws implements AWS Lambda deploy via the AWS SDK v2.
package aws

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	iamv2 "github.com/aws/aws-sdk-go-v2/service/iam"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

func (p *Provider) Deploy(ctx context.Context, req sdkprovider.DeployRequest) (*sdkprovider.DeployResult, error) {
	cfg := req.Config
	stage := req.Stage
	root := req.Root

	service := sdkprovider.Service(cfg)
	if service == "" {
		return nil, fmt.Errorf("service is required in config")
	}

	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load AWS clients: %w", err)
	}

	roleARN := sdkprovider.Env("AWS_LAMBDA_ROLE_ARN")
	if roleARN == "" {
		roleARN, err = ensureLambdaExecRole(ctx, clients, service, stage)
		if err != nil {
			return nil, fmt.Errorf("ensure IAM execution role: %w", err)
		}
	}

	defaultRuntime := sdkprovider.ProviderRuntime(cfg)
	if defaultRuntime == "" {
		defaultRuntime = "nodejs20.x"
	}

	zipBytes, err := zipDeployDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("zip source directory: %w", err)
	}

	result := sdkprovider.BuildDeployResult(ProviderID, cfg, stage)
	result.Outputs["region"] = region
	enableFunctionURL := functionURLEnabled(cfg)

	functions := sdkprovider.Functions(cfg)
	for fnName, fn := range functions {
		handler := fn.Handler
		if handler == "" {
			handler = "index.handler"
		}
		fnRuntime := fn.Runtime
		if fnRuntime == "" {
			fnRuntime = defaultRuntime
		}
		fnRuntime = normalizeLambdaRuntime(fnRuntime)

		memory := int32(128)
		if fn.Memory > 0 {
			memory = int32(fn.Memory)
		}
		timeout := int32(30)
		if fn.Timeout > 0 {
			timeout = int32(fn.Timeout)
		}

		funcName := fmt.Sprintf("%s-%s-%s", service, stage, fnName)

		if err := deployLambdaFunction(ctx, clients, lambdaDeployInput{
			functionName: funcName,
			runtime:      fnRuntime,
			handler:      handler,
			roleARN:      roleARN,
			memory:       memory,
			timeout:      timeout,
			environment:  fn.Environment,
			zipBytes:     zipBytes,
		}); err != nil {
			return nil, err
		}

		arn := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", region, clients.AccountID, funcName)
		deployed := result.Functions[fnName]
		deployed.ResourceName = funcName
		deployed.ResourceIdentifier = arn
		deployed.Metadata = map[string]string{
			"region":      region,
			"accountID":   clients.AccountID,
			"functionArn": arn,
		}
		result.Functions[fnName] = deployed
		result.Outputs["arn_"+fnName] = arn

		if enableFunctionURL && fn.HasHTTP {
			url, _, urlErr := ensureFunctionURL(ctx, clients, funcName)
			if urlErr != nil {
				return nil, fmt.Errorf("ensure function URL %s: %w", funcName, urlErr)
			}
			result.Outputs["url_"+fnName] = url
			if len(functions) == 1 {
				result.Outputs["url"] = url
			}
		}
	}

	return result, nil
}

func functionURLEnabled(cfg sdkprovider.Config) bool {
	if asBool(sdkprovider.Env("RUNFABRIC_AWS_ENABLE_FUNCTION_URL")) {
		return true
	}
	provider, ok := cfg["provider"].(map[string]any)
	if !ok || len(provider) == 0 {
		return false
	}
	if asBool(provider["functionUrlEnabled"]) || asBool(provider["functionURLEnabled"]) {
		return true
	}
	if nested, ok := provider["functionUrl"].(map[string]any); ok {
		if asBool(nested["enabled"]) {
			return true
		}
	}
	if nested, ok := provider["functionURL"].(map[string]any); ok {
		if asBool(nested["enabled"]) {
			return true
		}
	}
	return false
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "1" || s == "true" || s == "yes" || s == "on"
	default:
		return false
	}
}

type lambdaDeployInput struct {
	functionName string
	runtime      string
	handler      string
	roleARN      string
	memory       int32
	timeout      int32
	environment  map[string]string
	zipBytes     []byte
}

func deployLambdaFunction(ctx context.Context, clients *AWSClients, in lambdaDeployInput) error {
	_, updateErr := clients.Lambda.UpdateFunctionCode(ctx, &lambdav2.UpdateFunctionCodeInput{
		FunctionName: awssdk.String(in.functionName),
		ZipFile:      in.zipBytes,
	})
	if updateErr != nil {
		if !isLambdaNotFound(updateErr) {
			return fmt.Errorf("UpdateFunctionCode %s: %w", in.functionName, updateErr)
		}
		if _, createErr := clients.Lambda.CreateFunction(ctx, &lambdav2.CreateFunctionInput{
			FunctionName: awssdk.String(in.functionName),
			Runtime:      lambdatypes.Runtime(in.runtime),
			Handler:      awssdk.String(in.handler),
			Role:         awssdk.String(in.roleARN),
			Environment:  lambdaEnvironment(in.environment),
			Code:         &lambdatypes.FunctionCode{ZipFile: in.zipBytes},
			Timeout:      awssdk.Int32(in.timeout),
			MemorySize:   awssdk.Int32(in.memory),
		}); createErr != nil {
			return fmt.Errorf("CreateFunction %s: %w", in.functionName, createErr)
		}
		if err := waitUntilFunctionReady(ctx, clients, in.functionName); err != nil {
			return fmt.Errorf("wait for created function %s: %w", in.functionName, err)
		}
		return nil
	}

	if err := waitUntilFunctionReady(ctx, clients, in.functionName); err != nil {
		return fmt.Errorf("wait for code update %s: %w", in.functionName, err)
	}

	_, err := clients.Lambda.UpdateFunctionConfiguration(ctx, &lambdav2.UpdateFunctionConfigurationInput{
		FunctionName: awssdk.String(in.functionName),
		Runtime:      lambdatypes.Runtime(in.runtime),
		Handler:      awssdk.String(in.handler),
		Role:         awssdk.String(in.roleARN),
		Environment:  lambdaEnvironment(in.environment),
		Timeout:      awssdk.Int32(in.timeout),
		MemorySize:   awssdk.Int32(in.memory),
	})
	if err != nil {
		return fmt.Errorf("UpdateFunctionConfiguration %s: %w", in.functionName, err)
	}
	if err := waitUntilFunctionReady(ctx, clients, in.functionName); err != nil {
		return fmt.Errorf("wait for config update %s: %w", in.functionName, err)
	}
	return nil
}

func lambdaEnvironment(values map[string]string) *lambdatypes.Environment {
	if len(values) == 0 {
		return nil
	}
	vars := make(map[string]string, len(values))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		vars[trimmedKey] = value
	}
	if len(vars) == 0 {
		return nil
	}
	return &lambdatypes.Environment{Variables: vars}
}

// normalizeLambdaRuntime maps short runtime names to Lambda identifier strings.
func normalizeLambdaRuntime(runtime string) string {
	switch strings.ToLower(runtime) {
	case "nodejs", "node":
		return "nodejs20.x"
	case "python":
		return "python3.12"
	case "go", "golang":
		return "provided.al2023"
	default:
		return runtime
	}
}

// ensureLambdaExecRole creates an IAM execution role for Lambda if it does not
// already exist, and returns its ARN. The role name is runfabric-{service}-{stage}-exec.
func ensureLambdaExecRole(ctx context.Context, clients *AWSClients, service, stage string) (string, error) {
	roleName := fmt.Sprintf("runfabric-%s-%s-exec", service, stage)

	getOut, err := clients.IAM.GetRole(ctx, &iamv2.GetRoleInput{RoleName: awssdk.String(roleName)})
	if err == nil {
		return awssdk.ToString(getOut.Role.Arn), nil
	}
	if !isIAMNoSuchEntity(err) {
		return "", fmt.Errorf("GetRole %s: %w", roleName, err)
	}

	assumeDoc, _ := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":    "Allow",
			"Principal": map[string]any{"Service": "lambda.amazonaws.com"},
			"Action":    "sts:AssumeRole",
		}},
	})
	createOut, err := clients.IAM.CreateRole(ctx, &iamv2.CreateRoleInput{
		RoleName:                 awssdk.String(roleName),
		AssumeRolePolicyDocument: awssdk.String(string(assumeDoc)),
	})
	if err != nil {
		return "", fmt.Errorf("CreateRole %s: %w", roleName, err)
	}
	if _, err := clients.IAM.AttachRolePolicy(ctx, &iamv2.AttachRolePolicyInput{
		RoleName:  awssdk.String(roleName),
		PolicyArn: awssdk.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	}); err != nil {
		return "", fmt.Errorf("AttachRolePolicy %s: %w", roleName, err)
	}

	// Allow IAM role to propagate before Lambda can assume it.
	_ = retry(ctx, 6, 5*time.Second, func() error {
		_, testErr := clients.Lambda.GetFunction(ctx, &lambdav2.GetFunctionInput{
			FunctionName: awssdk.String("runfabric-probe-" + stage),
		})
		// Any response (including NotFound) means credentials/IAM are reachable.
		if testErr == nil || isLambdaNotFound(testErr) {
			return nil
		}
		return testErr
	})

	return awssdk.ToString(createOut.Role.Arn), nil
}

// zipDeployDirectory creates an in-memory ZIP of root, excluding node_modules and .git.
func zipDeployDirectory(root string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		sep := string(filepath.Separator)
		if strings.Contains(path, "node_modules"+sep) || strings.Contains(path, ".git"+sep) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		f, err := w.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
