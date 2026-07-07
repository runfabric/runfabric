package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Invoke calls the deployed Lambda synchronously and returns its response
// payload. The deployed function name follows the deploy convention
// "<service>-<stage>-<function>".
func (p *Provider) Invoke(ctx context.Context, req sdkprovider.InvokeRequest) (*sdkprovider.InvokeResult, error) {
	if req.Function == "" {
		return nil, fmt.Errorf("invoke requires a function name")
	}
	cfg := req.Config
	stage := req.Stage
	service := sdkprovider.Service(cfg)
	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}

	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	functionName := fmt.Sprintf("%s-%s-%s", service, stage, req.Function)
	payload := req.Payload
	if len(payload) == 0 {
		payload = []byte("{}") // Lambda rejects an empty body; default to {}.
	}

	out, err := clients.Lambda.Invoke(ctx, &lambdav2.InvokeInput{
		FunctionName: awssdk.String(functionName),
		Payload:      payload,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke %s: %w", functionName, err)
	}
	// A handled function error comes back as 200 with FunctionError set and the
	// error object as the payload — surface it instead of reporting success.
	if out.FunctionError != nil && *out.FunctionError != "" {
		return nil, fmt.Errorf("function %s returned %s: %s", functionName, *out.FunctionError, string(out.Payload))
	}

	return &sdkprovider.InvokeResult{
		Provider: p.Name(),
		Function: req.Function,
		Output:   string(out.Payload),
	}, nil
}
