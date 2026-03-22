package aws

import (
	"context"
	"time"

	sfnv2 "github.com/aws/aws-sdk-go-v2/service/sfn"
	providers "github.com/runfabric/runfabric/platform/core/contracts/extension/provider"
	extruntimes "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	"github.com/runfabric/runfabric/platform/core/model/config"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	planner "github.com/runfabric/runfabric/platform/core/planner/engine"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/extensions/internal/runtimes"
)

func (p *Provider) Plan(ctx context.Context, req providers.PlanRequest) (*providers.PlanResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cfg := req.Config
	stage := req.Stage
	root := req.Root

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "load aws config failed during plan", err)
	}

	artifactMap := map[string]providers.Artifact{}
	runtimeRegistry := runtimes.NewBuiltinRegistry()
	for fnName, fn := range cfg.Functions {
		configSig, err := buildConfigSignature(fn)
		if err != nil {
			return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "build config signature failed: "+fnName, err)
		}
		rt, err := runtimeRegistry.Get(fn.Runtime)
		if err != nil {
			return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "resolve runtime plugin failed: "+fnName, err)
		}
		artifact, err := rt.Build(ctx, extruntimes.BuildRequest{
			Root:            root,
			FunctionName:    fnName,
			FunctionConfig:  fn,
			ConfigSignature: configSig,
		})
		if err != nil {
			return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "package function during plan failed: "+fnName, err)
		}
		artifactMap[fnName] = *artifact
	}

	actual, err := discoverActualState(ctx, clients, cfg, stage)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "discover actual state failed", err)
	}

	desired := desiredStateFromConfig(cfg, stage, artifactMap)
	plan := planner.Diff(desired, actual, cfg.Service, stage, cfg.Provider.Name)

	receipt, _ := state.Load(root, stage)
	receiptMap := buildReceiptFunctionMap(receipt)
	actualMap := actualFunctionMap(actual)

	for i := range plan.Actions {
		action := &plan.Actions[i]
		if action.Resource != planner.ResourceFunction {
			continue
		}

		fnConfigName := reverseFunctionName(cfg, stage, action.Name)
		if fnConfigName == "" {
			continue
		}

		fn, ok := cfg.Functions[fnConfigName]
		if !ok {
			continue
		}
		artifact := artifactMap[fnConfigName]

		desiredFunction := planner.DesiredFunction{
			Name:            functionName(cfg, stage, fnConfigName),
			Runtime:         fn.Runtime,
			Handler:         fn.Handler,
			Memory:          defaultInt(fn.Memory, 128),
			Timeout:         defaultInt(fn.Timeout, 10),
			CodeSHA256:      artifact.SHA256,
			ConfigSignature: artifact.ConfigSignature,
		}

		var actualFunctionPtr *planner.ActualFunction
		if actualFunction, ok := actualMap[action.Name]; ok {
			actualFunctionPtr = &actualFunction
		}

		changeSet := detectFunctionChange(fnConfigName, artifact, desiredFunction, actualFunctionPtr, receiptMap)

		switch {
		case changeSet.NeedsCreate:
			action.Type = planner.ActionCreate
			action.Description = "Create function"
			action.Metadata = mergeMap(action.Metadata, map[string]string{
				"reason": changeSet.Reason,
			})
		case changeSet.NeedsUpdate:
			action.Type = planner.ActionUpdate
			action.Description = "Update function"
			action.Metadata = mergeMap(action.Metadata, map[string]string{
				"reason":           changeSet.Reason,
				"remoteCodeChange": boolString(changeSet.RemoteCodeChange),
				"remoteCfgChange":  boolString(changeSet.RemoteCfgChange),
			})
			action.Changes = map[string][2]string{
				"desiredArtifactSha": {"remote/current", artifact.SHA256},
				"desiredConfigSig":   {"remote/current", artifact.ConfigSignature},
			}
		default:
			action.Type = planner.ActionNoop
			action.Description = "Function unchanged"
			action.Metadata = mergeMap(action.Metadata, map[string]string{
				"reason": changeSet.Reason,
			})
		}
	}

	warnings := []string{}
	if actual.HTTPAPI == nil && desired.HTTPAPI != nil {
		warnings = append(warnings, "HTTP API does not exist yet and will be created")
	}

	stepFunctions, sfErr := stepFunctionsFromConfig(cfg, root)
	if sfErr != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "step functions config parse failed", sfErr)
	}
	if len(stepFunctions) > 0 {
		existing := map[string]string{}
		listOut, err := clients.SFN.ListStateMachines(ctx, &sfnv2.ListStateMachinesInput{})
		if err != nil {
			warnings = append(warnings, "Step Functions discovery failed: "+err.Error())
		} else {
			for _, sm := range listOut.StateMachines {
				if sm.Name != nil && sm.StateMachineArn != nil {
					existing[*sm.Name] = *sm.StateMachineArn
				}
			}
		}

		desiredNames := map[string]struct{}{}
		for _, decl := range stepFunctions {
			desiredNames[decl.Name] = struct{}{}
			action := planner.PlanAction{
				ID:          "stepfunction:" + decl.Name,
				Name:        decl.Name,
				Resource:    planner.ResourceIntegration,
				Description: "Step Functions state machine",
				Metadata: map[string]string{
					"source":   "extensions.aws-lambda.stepFunctions",
					"category": "orchestration",
					"kind":     "step_functions",
				},
			}
			if arn, ok := existing[decl.Name]; ok {
				action.Type = planner.ActionUpdate
				action.Description = "Update Step Functions state machine"
				action.Metadata["arn"] = arn
			} else {
				action.Type = planner.ActionCreate
				action.Description = "Create Step Functions state machine"
			}
			plan.Actions = append(plan.Actions, action)
		}

		for name, arn := range existing {
			if _, ok := desiredNames[name]; ok {
				continue
			}
			plan.Actions = append(plan.Actions, planner.PlanAction{
				ID:          "stepfunction:delete:" + name,
				Type:        planner.ActionDelete,
				Resource:    planner.ResourceIntegration,
				Name:        name,
				Description: "Delete Step Functions state machine",
				Metadata: map[string]string{
					"arn":      arn,
					"source":   "extensions.aws-lambda.stepFunctions",
					"category": "orchestration",
					"kind":     "step_functions",
				},
			})
		}
	}

	return &providers.PlanResult{
		Provider: p.Name(),
		Plan:     plan,
		Warnings: warnings,
	}, nil
}

func reverseFunctionName(cfg *config.Config, stage, deployedName string) string {
	for fnName := range cfg.Functions {
		if functionName(cfg, stage, fnName) == deployedName {
			return fnName
		}
	}
	return ""
}

func mergeMap(a, b map[string]string) map[string]string {
	if a == nil && b == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
