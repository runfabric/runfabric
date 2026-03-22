package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	sfnv2 "github.com/aws/aws-sdk-go-v2/service/sfn"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

func (p *Provider) Invoke(ctx context.Context, req providers.InvokeRequest) (*providers.InvokeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cfg := req.Config
	stage := req.Stage
	function := req.Function
	payload := req.Payload

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(function, "sfn:") || strings.HasPrefix(function, "stepfunction:") {
		machineName := strings.TrimPrefix(strings.TrimPrefix(function, "sfn:"), "stepfunction:")
		if strings.TrimSpace(machineName) == "" {
			return nil, fmt.Errorf("state machine name is required (use sfn:<name>)")
		}
		stepFunctions, err := stepFunctionsFromConfig(cfg, ".")
		if err != nil {
			return nil, err
		}
		declared := false
		for _, decl := range stepFunctions {
			if decl.Name == machineName {
				declared = true
				break
			}
		}
		if !declared {
			return nil, fmt.Errorf("state machine %q not declared in extensions.aws-lambda.stepFunctions", machineName)
		}

		listOut, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
		if err != nil {
			return nil, err
		}
		var arn *string
		for _, sm := range listOut.StateMachines {
			if sm.Name != nil && *sm.Name == machineName {
				arn = sm.StateMachineArn
				break
			}
		}
		if arn == nil {
			return nil, fmt.Errorf("state machine %q not found in AWS account/region", machineName)
		}

		input := string(payload)
		if strings.TrimSpace(input) == "" {
			input = "{}"
		}
		startOut, err := clients.SFN.StartExecution(ctx, &sfnv2.StartExecutionInput{
			StateMachineArn: arn,
			Input:           &input,
		})
		if err != nil {
			return nil, err
		}
		runID := ""
		if startOut.ExecutionArn != nil {
			runID = *startOut.ExecutionArn
		}
		return &providers.InvokeResult{
			Provider: p.Name(),
			Function: function,
			Output:   fmt.Sprintf("started Step Functions execution for %s", machineName),
			RunID:    runID,
			Workflow: machineName,
		}, nil
	}

	name := functionName(cfg, stage, function)

	out, err := clients.Lambda.Invoke(ctx, &lambdav2.InvokeInput{
		FunctionName: &name,
		Payload:      payload,
	})
	if err != nil {
		return nil, err
	}

	return &providers.InvokeResult{
		Provider: p.Name(),
		Function: function,
		Output:   string(out.Payload),
	}, nil
}
