package aws

import (
	"context"
	"fmt"
	"strings"

	sfnv2 "github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sfn/types"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
)

func (p *Provider) SyncOrchestrations(ctx context.Context, req providers.OrchestrationSyncRequest) (*providers.OrchestrationSyncResult, error) {
	decls, err := stepFunctionsFromConfig(req.Config, req.Root)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return &providers.OrchestrationSyncResult{}, nil
	}
	clients, err := loadClients(ctx, req.Config.Provider.Region)
	if err != nil {
		return nil, err
	}

	existingByName := map[string]types.StateMachineListItem{}
	listOut, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
	if err != nil {
		return nil, err
	}
	for _, sm := range listOut.StateMachines {
		if sm.Name != nil {
			existingByName[*sm.Name] = sm
		}
	}

	res := &providers.OrchestrationSyncResult{Metadata: map[string]string{}, Outputs: map[string]string{}}
	for _, decl := range decls {
		def, err := stepFunctionDefinitionString(req.Root, decl)
		if err != nil {
			return nil, err
		}
		def = applyStepFunctionBindings(def, decl, req.FunctionResourceByName)
		if existing, ok := existingByName[decl.Name]; ok && existing.StateMachineArn != nil {
			in := &sfnv2.UpdateStateMachineInput{StateMachineArn: existing.StateMachineArn, Definition: &def}
			if strings.TrimSpace(decl.Role) != "" {
				in.RoleArn = &decl.Role
			}
			if _, err := clients.SFN.UpdateStateMachine(ctx, in); err != nil {
				return nil, err
			}
			arn := *existing.StateMachineArn
			res.Metadata["stepfunction:"+decl.Name+":arn"] = arn
			res.Metadata["stepfunction:"+decl.Name+":operation"] = "updated"
			res.Metadata["stepfunction:"+decl.Name+":console"] = stateMachineConsoleLink(req.Config.Provider.Region, arn)
			continue
		}
		if strings.TrimSpace(decl.Role) == "" {
			return nil, fmt.Errorf("stepFunctions %q requires role when creating a state machine", decl.Name)
		}
		createOut, err := clients.SFN.CreateStateMachine(ctx, &sfnv2.CreateStateMachineInput{
			Name:       &decl.Name,
			Definition: &def,
			RoleArn:    &decl.Role,
		})
		if err != nil {
			return nil, err
		}
		arn := ""
		if createOut.StateMachineArn != nil {
			arn = *createOut.StateMachineArn
		}
		res.Metadata["stepfunction:"+decl.Name+":arn"] = arn
		res.Metadata["stepfunction:"+decl.Name+":operation"] = "created"
		res.Metadata["stepfunction:"+decl.Name+":console"] = stateMachineConsoleLink(req.Config.Provider.Region, arn)
	}
	return res, nil
}

func (p *Provider) RemoveOrchestrations(ctx context.Context, req providers.OrchestrationRemoveRequest) (*providers.OrchestrationSyncResult, error) {
	decls, err := stepFunctionsFromConfig(req.Config, req.Root)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return &providers.OrchestrationSyncResult{}, nil
	}
	clients, err := loadClients(ctx, req.Config.Provider.Region)
	if err != nil {
		return nil, err
	}
	listOut, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
	if err != nil {
		return nil, err
	}
	arnByName := map[string]string{}
	for _, sm := range listOut.StateMachines {
		if sm.Name != nil && sm.StateMachineArn != nil {
			arnByName[*sm.Name] = *sm.StateMachineArn
		}
	}
	res := &providers.OrchestrationSyncResult{Metadata: map[string]string{}, Outputs: map[string]string{}}
	for _, decl := range decls {
		arn, ok := arnByName[decl.Name]
		if !ok {
			res.Metadata["stepfunction:"+decl.Name+":operation"] = "absent"
			continue
		}
		_, err := clients.SFN.DeleteStateMachine(ctx, &sfnv2.DeleteStateMachineInput{StateMachineArn: &arn})
		if err != nil {
			return nil, err
		}
		res.Metadata["stepfunction:"+decl.Name+":arn"] = arn
		res.Metadata["stepfunction:"+decl.Name+":operation"] = "deleted"
	}
	return res, nil
}

func (p *Provider) InvokeOrchestration(ctx context.Context, req providers.OrchestrationInvokeRequest) (*providers.InvokeResult, error) {
	clients, err := loadClients(ctx, req.Config.Provider.Region)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("orchestration name is required")
	}
	listOut, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
	if err != nil {
		return nil, err
	}
	var arn string
	for _, sm := range listOut.StateMachines {
		if sm.Name != nil && *sm.Name == name && sm.StateMachineArn != nil {
			arn = *sm.StateMachineArn
			break
		}
	}
	if arn == "" {
		return nil, fmt.Errorf("state machine %q not found", name)
	}
	input := string(req.Payload)
	if strings.TrimSpace(input) == "" {
		input = "{}"
	}
	out, err := clients.SFN.StartExecution(ctx, &sfnv2.StartExecutionInput{StateMachineArn: &arn, Input: &input})
	if err != nil {
		return nil, err
	}
	runID := ""
	if out.ExecutionArn != nil {
		runID = *out.ExecutionArn
	}
	executionStatus := "RUNNING"
	if runID != "" {
		if execOut, err := clients.SFN.DescribeExecution(ctx, &sfnv2.DescribeExecutionInput{ExecutionArn: &runID}); err == nil {
			executionStatus = string(execOut.Status)
		}
	}
	console := executionConsoleLink(req.Config.Provider.Region, runID)
	output := "started Step Functions execution"
	if executionStatus != "" {
		output += " (status=" + executionStatus + ")"
	}
	if console != "" {
		output += " " + console
	}
	return &providers.InvokeResult{
		Provider: p.Name(),
		Function: "sfn:" + name,
		Output:   output,
		RunID:    runID,
		Workflow: name,
	}, nil
}

func (p *Provider) InspectOrchestrations(ctx context.Context, req providers.OrchestrationInspectRequest) (map[string]any, error) {
	decls, err := stepFunctionsFromConfig(req.Config, req.Root)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return map[string]any{"stepFunctions": []any{}}, nil
	}
	clients, err := loadClients(ctx, req.Config.Provider.Region)
	if err != nil {
		return nil, err
	}
	listOut, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
	if err != nil {
		return nil, err
	}
	arnByName := map[string]string{}
	for _, sm := range listOut.StateMachines {
		if sm.Name != nil && sm.StateMachineArn != nil {
			arnByName[*sm.Name] = *sm.StateMachineArn
		}
	}
	items := make([]map[string]any, 0, len(decls))
	for _, decl := range decls {
		item := map[string]any{"name": decl.Name, "declared": true}
		if arn, ok := arnByName[decl.Name]; ok {
			item["arn"] = arn
			item["console"] = stateMachineConsoleLink(req.Config.Provider.Region, arn)
			descOut, err := clients.SFN.DescribeStateMachine(ctx, &sfnv2.DescribeStateMachineInput{StateMachineArn: &arn})
			if err == nil && descOut.Status != "" {
				item["status"] = string(descOut.Status)
			}
			execOut, err := clients.SFN.ListExecutions(ctx, &sfnv2.ListExecutionsInput{StateMachineArn: &arn, MaxResults: 1})
			if err == nil && len(execOut.Executions) > 0 {
				exec := execOut.Executions[0]
				if exec.ExecutionArn != nil {
					item["latestExecutionArn"] = *exec.ExecutionArn
					item["latestExecutionConsole"] = executionConsoleLink(req.Config.Provider.Region, *exec.ExecutionArn)
					historyOut, hErr := clients.SFN.GetExecutionHistory(ctx, &sfnv2.GetExecutionHistoryInput{ExecutionArn: exec.ExecutionArn, MaxResults: 20})
					if hErr == nil {
						history := make([]map[string]any, 0, len(historyOut.Events))
						for _, ev := range historyOut.Events {
							entry := map[string]any{"id": ev.Id, "type": string(ev.Type)}
							if ev.Timestamp != nil {
								entry["timestamp"] = ev.Timestamp.String()
							}
							history = append(history, entry)
						}
						item["latestExecutionHistory"] = history
					}
				}
				item["latestExecutionStatus"] = string(exec.Status)
			}
		} else {
			item["status"] = "absent"
		}
		items = append(items, item)
	}
	return map[string]any{"stepFunctions": items}, nil
}

func stateMachineConsoleLink(region, arn string) string {
	if strings.TrimSpace(region) == "" || strings.TrimSpace(arn) == "" {
		return ""
	}
	return "https://" + region + ".console.aws.amazon.com/states/home?region=" + region + "#/statemachines/view/" + arn
}

func executionConsoleLink(region, executionArn string) string {
	if strings.TrimSpace(region) == "" || strings.TrimSpace(executionArn) == "" {
		return ""
	}
	return "https://" + region + ".console.aws.amazon.com/states/home?region=" + region + "#/executions/details/" + executionArn
}
