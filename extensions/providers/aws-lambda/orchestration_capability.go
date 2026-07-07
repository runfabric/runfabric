package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	iamv2 "github.com/aws/aws-sdk-go-v2/service/iam"
	sfnv2 "github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Orchestration support maps configured workflows to AWS Step Functions state
// machines. A workflow is expressible as a state machine only when every step
// is a `code` step bound to a deployed function (input.function): each becomes
// a Lambda-invoke Task, chained in order. Workflows with AI / human-approval /
// unbound steps run on the RunFabric durable runtime instead and are skipped
// here with a recorded reason (rather than being misrepresented as a machine
// that silently drops those steps).

// failState is the shared terminal state that steps route unrecovered errors to.
const failState = "runfabric-failed"

type wfStep struct {
	id       string
	kind     string
	function string
}

type wfDef struct {
	name  string
	steps []wfStep
}

// orchestrationWorkflows extracts workflow definitions from the config, tolerant
// of both the yaml (lowercase) and Go-marshaled (PascalCase) key spellings.
func orchestrationWorkflows(cfg sdkprovider.Config) []wfDef {
	raw, ok := pick(cfg, "workflows", "Workflows").([]any)
	if !ok {
		return nil
	}
	var out []wfDef
	for _, w := range raw {
		m, ok := w.(map[string]any)
		if !ok {
			continue
		}
		name, _ := pick(m, "name", "Name").(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		var steps []wfStep
		sraw, _ := pick(m, "steps", "Steps").([]any)
		for i, s := range sraw {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			id, _ := pick(sm, "id", "ID").(string)
			if strings.TrimSpace(id) == "" {
				id = fmt.Sprintf("step%d", i+1)
			}
			kind, _ := pick(sm, "kind", "Kind").(string)
			fn := ""
			if input, ok := pick(sm, "input", "Input").(map[string]any); ok {
				fn, _ = pick(input, "function", "Function").(string)
			}
			steps = append(steps, wfStep{id: id, kind: strings.ToLower(strings.TrimSpace(kind)), function: strings.TrimSpace(fn)})
		}
		out = append(out, wfDef{name: name, steps: steps})
	}
	return out
}

func pick(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// buildASL renders a linear Amazon States Language definition for wf, or reports
// why it cannot be expressed as a state machine.
func buildASL(wf wfDef, arnByFunction map[string]string) (definition string, ok bool, reason string) {
	if len(wf.steps) == 0 {
		return "", false, "workflow has no steps"
	}
	states := map[string]any{}
	for i, st := range wf.steps {
		if st.kind != "code" {
			return "", false, fmt.Sprintf("step %q kind %q is not a Step Functions task (runs on the durable runtime)", st.id, st.kind)
		}
		if st.function == "" {
			return "", false, fmt.Sprintf("step %q has no input.function to invoke", st.id)
		}
		arn := arnByFunction[st.function]
		if arn == "" {
			return "", false, fmt.Sprintf("step %q function %q is not deployed", st.id, st.function)
		}
		state := map[string]any{
			"Type":     "Task",
			"Resource": "arn:aws:states:::lambda:invoke",
			"Parameters": map[string]any{
				"FunctionName": arn,
				"Payload.$":    "$",
			},
			"OutputPath": "$.Payload",
			// Retry transient Lambda/service faults with exponential backoff, and
			// route any unrecovered failure to a terminal Fail state.
			"Retry": []any{map[string]any{
				"ErrorEquals":     []string{"Lambda.ServiceException", "Lambda.TooManyRequestsException", "Lambda.SdkClientException", "States.TaskFailed"},
				"IntervalSeconds": 2,
				"MaxAttempts":     3,
				"BackoffRate":     2.0,
			}},
			"Catch": []any{map[string]any{
				"ErrorEquals": []string{"States.ALL"},
				"Next":        failState,
			}},
		}
		if i == len(wf.steps)-1 {
			state["End"] = true
		} else {
			state["Next"] = wf.steps[i+1].id
		}
		states[st.id] = state
	}
	states[failState] = map[string]any{
		"Type":  "Fail",
		"Error": "WorkflowStepFailed",
		"Cause": "a workflow step failed after retries",
	}
	def := map[string]any{
		"Comment": "runfabric workflow " + wf.name,
		"StartAt": wf.steps[0].id,
		"States":  states,
	}
	b, err := json.Marshal(def)
	if err != nil {
		return "", false, fmt.Sprintf("marshal state machine: %v", err)
	}
	return string(b), true, ""
}

// SyncOrchestrations creates/updates a Step Functions state machine per
// expressible workflow (name <service>-<stage>-<workflow>), backed by a shared
// role that may invoke the stage's Lambdas.
func (p *Provider) SyncOrchestrations(ctx context.Context, req sdkprovider.OrchestrationSyncRequest) (*sdkprovider.OrchestrationSyncResult, error) {
	cfg := req.Config
	stage := req.Stage
	service := sdkprovider.Service(cfg)
	workflows := orchestrationWorkflows(cfg)
	if len(workflows) == 0 {
		return &sdkprovider.OrchestrationSyncResult{
			Metadata: map[string]string{"provider": ProviderID, "status": "no-workflows"},
			Outputs:  map[string]string{},
		}, nil
	}
	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}
	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	roleArn, err := ensureSfnRole(ctx, clients, service, stage)
	if err != nil {
		return nil, fmt.Errorf("ensure step functions role: %w", err)
	}
	existing, err := stateMachineARNs(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("list state machines: %w", err)
	}

	outputs := map[string]string{}
	metadata := map[string]string{"provider": ProviderID}
	synced := 0
	for _, wf := range workflows {
		def, ok, reason := buildASL(wf, req.FunctionResourceByName)
		if !ok {
			metadata["skipped:"+wf.name] = reason
			continue
		}
		smName := fmt.Sprintf("%s-%s-%s", service, stage, wf.name)
		if arn, exists := existing[smName]; exists {
			if _, err := clients.SFN.UpdateStateMachine(ctx, &sfnv2.UpdateStateMachineInput{
				StateMachineArn: awssdk.String(arn),
				Definition:      awssdk.String(def),
				RoleArn:         awssdk.String(roleArn),
			}); err != nil {
				return nil, fmt.Errorf("update state machine %s: %w", smName, err)
			}
			outputs[wf.name] = arn
		} else {
			out, err := clients.SFN.CreateStateMachine(ctx, &sfnv2.CreateStateMachineInput{
				Name:       awssdk.String(smName),
				Definition: awssdk.String(def),
				RoleArn:    awssdk.String(roleArn),
				Type:       sfntypes.StateMachineTypeStandard,
			})
			if err != nil {
				return nil, fmt.Errorf("create state machine %s: %w", smName, err)
			}
			outputs[wf.name] = awssdk.ToString(out.StateMachineArn)
		}
		synced++
	}
	metadata["status"] = "synced"
	metadata["synced"] = fmt.Sprintf("%d", synced)
	return &sdkprovider.OrchestrationSyncResult{Metadata: metadata, Outputs: outputs}, nil
}

// RemoveOrchestrations deletes every state machine for the stage and the shared
// Step Functions role.
func (p *Provider) RemoveOrchestrations(ctx context.Context, req sdkprovider.OrchestrationRemoveRequest) (*sdkprovider.OrchestrationSyncResult, error) {
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

	existing, err := stateMachineARNs(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("list state machines: %w", err)
	}
	prefix := fmt.Sprintf("%s-%s-", service, stage)
	removed := 0
	var errs []string
	for name, arn := range existing {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if _, err := clients.SFN.DeleteStateMachine(ctx, &sfnv2.DeleteStateMachineInput{
			StateMachineArn: awssdk.String(arn),
		}); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		} else {
			removed++
		}
	}
	roleName := fmt.Sprintf("runfabric-%s-%s-sfn", service, stage)
	if err := deleteExecRole(ctx, clients, roleName); err != nil {
		errs = append(errs, fmt.Sprintf("role %s: %v", roleName, err))
	}

	metadata := map[string]string{"provider": ProviderID, "status": "removed", "removed": fmt.Sprintf("%d", removed)}
	if len(errs) > 0 {
		metadata["errors"] = strings.Join(errs, "; ")
	}
	return &sdkprovider.OrchestrationSyncResult{Metadata: metadata, Outputs: map[string]string{}}, nil
}

// InvokeOrchestration starts an execution of the named workflow's state machine.
func (p *Provider) InvokeOrchestration(ctx context.Context, req sdkprovider.OrchestrationInvokeRequest) (*sdkprovider.InvokeResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("orchestration name is required")
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

	existing, err := stateMachineARNs(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("list state machines: %w", err)
	}
	smName := fmt.Sprintf("%s-%s-%s", service, stage, name)
	arn, ok := existing[smName]
	if !ok {
		return nil, fmt.Errorf("orchestration %q is not deployed (no state machine %s)", name, smName)
	}
	input := "{}"
	if len(req.Payload) > 0 {
		input = string(req.Payload)
	}
	out, err := clients.SFN.StartExecution(ctx, &sfnv2.StartExecutionInput{
		StateMachineArn: awssdk.String(arn),
		Input:           awssdk.String(input),
	})
	if err != nil {
		return nil, fmt.Errorf("start execution %s: %w", smName, err)
	}
	return &sdkprovider.InvokeResult{
		Provider: p.Name(),
		Function: "sfn:" + name,
		Output:   awssdk.ToString(out.ExecutionArn),
		Workflow: name,
	}, nil
}

// InspectOrchestrations lists the stage's state machines and their recent
// executions.
func (p *Provider) InspectOrchestrations(ctx context.Context, req sdkprovider.OrchestrationInspectRequest) (map[string]any, error) {
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

	existing, err := stateMachineARNs(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("list state machines: %w", err)
	}
	prefix := fmt.Sprintf("%s-%s-", service, stage)
	machines := []any{}
	for name, arn := range existing {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		item := map[string]any{"name": name, "arn": arn}
		if execs, err := clients.SFN.ListExecutions(ctx, &sfnv2.ListExecutionsInput{
			StateMachineArn: awssdk.String(arn),
			MaxResults:      int32(5),
		}); err == nil {
			runs := make([]any, 0, len(execs.Executions))
			for _, e := range execs.Executions {
				runs = append(runs, map[string]any{
					"arn":    awssdk.ToString(e.ExecutionArn),
					"name":   awssdk.ToString(e.Name),
					"status": string(e.Status),
				})
			}
			item["executions"] = runs
		}
		machines = append(machines, item)
	}
	return map[string]any{"stateMachines": machines}, nil
}

// ensureSfnRole creates (idempotently) a role that Step Functions assumes to
// invoke the stage's Lambda functions, returning its ARN.
func ensureSfnRole(ctx context.Context, clients *AWSClients, service, stage string) (string, error) {
	roleName := fmt.Sprintf("runfabric-%s-%s-sfn", service, stage)
	if out, err := clients.IAM.GetRole(ctx, &iamv2.GetRoleInput{RoleName: awssdk.String(roleName)}); err == nil {
		return awssdk.ToString(out.Role.Arn), nil
	} else if !isIAMNoSuchEntity(err) {
		return "", fmt.Errorf("GetRole %s: %w", roleName, err)
	}

	assume, _ := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":    "Allow",
			"Principal": map[string]any{"Service": "states.amazonaws.com"},
			"Action":    "sts:AssumeRole",
		}},
	})
	out, err := clients.IAM.CreateRole(ctx, &iamv2.CreateRoleInput{
		RoleName:                 awssdk.String(roleName),
		AssumeRolePolicyDocument: awssdk.String(string(assume)),
	})
	if err != nil {
		return "", fmt.Errorf("CreateRole %s: %w", roleName, err)
	}
	policy, _ := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":   "Allow",
			"Action":   []string{"lambda:InvokeFunction"},
			"Resource": "*",
		}},
	})
	if _, err := clients.IAM.PutRolePolicy(ctx, &iamv2.PutRolePolicyInput{
		RoleName:       awssdk.String(roleName),
		PolicyName:     awssdk.String("invoke-lambda"),
		PolicyDocument: awssdk.String(string(policy)),
	}); err != nil {
		return "", fmt.Errorf("PutRolePolicy %s: %w", roleName, err)
	}
	return awssdk.ToString(out.Role.Arn), nil
}

// stateMachineARNs returns a name→ARN map of all state machines in the account.
func stateMachineARNs(ctx context.Context, clients *AWSClients) (map[string]string, error) {
	out := map[string]string{}
	var next *string
	for {
		res, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{NextToken: next})
		if err != nil {
			return nil, err
		}
		for _, sm := range res.StateMachines {
			out[awssdk.ToString(sm.Name)] = awssdk.ToString(sm.StateMachineArn)
		}
		if res.NextToken == nil {
			return out, nil
		}
		next = res.NextToken
	}
}
