package aws

import (
	"context"
	"fmt"
	"time"

	extruntimes "github.com/runfabric/runfabric/platform/core/contracts/runtime"
	"github.com/runfabric/runfabric/platform/core/model/config"
	appErrs "github.com/runfabric/runfabric/platform/core/model/errors"
	state "github.com/runfabric/runfabric/platform/core/state/core"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
	deployexec "github.com/runfabric/runfabric/platform/deploy/exec"
	"github.com/runfabric/runfabric/platform/extensions/internal/runtimes"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

	awstriggers "github.com/runfabric/runfabric/platform/extensions/internal/providers/aws/triggers"
	planner "github.com/runfabric/runfabric/platform/planner/engine"
)

func ResumeDeploy(
	ctx context.Context,
	cfg *config.Config,
	stage string,
	root string,
	jf *transactions.JournalFile,
) (*map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	journalBackend := transactions.NewFileBackend(root)
	journal := transactions.NewJournal(cfg.Service, stage, jf.Operation, journalBackend)
	journal.File().Status = jf.Status
	journal.File().Operations = jf.Operations
	journal.File().Checkpoints = jf.Checkpoints
	journal.File().StartedAt = jf.StartedAt
	journal.File().UpdatedAt = jf.UpdatedAt

	deps, err := newResumeDependencies(ctx, root, cfg, stage, journal)
	if err != nil {
		return nil, err
	}

	engine := newDeployEngine(cfg, stage, root, deps)
	execCtx := &deployexec.Context{
		Root:      root,
		Config:    cfg,
		Stage:     stage,
		Artifacts: map[string]sdkprovider.Artifact{},
		Receipt:   deps.Receipt,
		Outputs:   map[string]string{},
		Metadata:  map[string]string{},
	}

	if err := engine.Run(ctx, execCtx); err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "resume deploy engine failed", err)
	}

	functions := make([]state.FunctionDeployment, 0, len(execCtx.Artifacts))
	for _, a := range execCtx.Artifacts {
		fn := state.FunctionDeployment{
			Function:        a.Function,
			ArtifactSHA256:  a.SHA256,
			ConfigSignature: a.ConfigSignature,
		}
		if execCtx.Metadata != nil {
			fn.ResourceName = execCtx.Metadata["lambda:"+a.Function+":name"]
			fn.ResourceIdentifier = execCtx.Metadata["lambda:"+a.Function+":arn"]
		}
		functions = append(functions, fn)
	}

	receipt := &state.Receipt{
		Service:      cfg.Service,
		Stage:        stage,
		Provider:     "aws-lambda",
		DeploymentID: fmt.Sprintf("aws-lambda-%s-%d", stage, time.Now().Unix()),
		Outputs:      execCtx.Outputs,
		Artifacts:    stateArtifactsFromProvider(artifactsFromMap(execCtx.Artifacts)),
		Metadata:     execCtx.Metadata,
		Functions:    functions,
	}
	state.EnrichReceiptWithWorkflows(receipt, cfg)

	if err := journal.Checkpoint(deployexec.CheckpointSaveReceipt, "in_progress"); err != nil {
		return nil, err
	}
	if err := state.Save(root, receipt); err != nil {
		return nil, err
	}
	if err := journal.Checkpoint(deployexec.CheckpointSaveReceipt, "done"); err != nil {
		return nil, err
	}

	if err := journal.MarkCompleted(); err != nil {
		return nil, err
	}
	_ = journal.Delete()

	result := map[string]any{
		"recovered": true,
		"mode":      "resume",
		"status":    "completed",
		"outputs":   execCtx.Outputs,
		"metadata":  execCtx.Metadata,
	}
	return &result, nil
}

type ResumeDependencies struct {
	Clients       *AWSClients
	Journal       *transactions.Journal
	Receipt       *state.Receipt
	ReceiptMap    map[string]state.FunctionDeployment
	ActualMap     map[string]planner.ActualFunction
	HTTPAPI       *HTTPAPI
	ArtifactOrder []string
}

func newDeployEngine(
	cfg *config.Config,
	stage string,
	root string,
	deps *ResumeDependencies,
) *deployexec.Engine {
	phases := []deployexec.Phase{
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointPackageArtifacts,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				runtimeRegistry := runtimes.NewBuiltinRegistry()
				for fnName, fn := range cfg.Functions {
					configSig, err := buildConfigSignature(fn)
					if err != nil {
						return err
					}

					rt, err := runtimeRegistry.Get(fn.Runtime)
					if err != nil {
						return err
					}
					artifact, err := rt.Build(ctx, extruntimes.BuildRequest{
						Root:            root,
						FunctionName:    fnName,
						FunctionConfig:  fn,
						ConfigSignature: configSig,
					})
					if err != nil {
						return err
					}

					ectx.Artifacts[fnName] = sdkArtifactFromCore(*artifact)
				}
				return nil
			},
		},
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointDiscoverState,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				actual, err := discoverActualState(ctx, deps.Clients, cfg, stage)
				if err != nil {
					return err
				}
				ectx.Actual = actual
				ectx.Desired = desiredStateFromConfig(cfg, stage, ectx.Artifacts)
				deps.ActualMap = actualFunctionMap(actual)
				return nil
			},
		},
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointEnsureHTTPAPI,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				if ectx.Desired == nil || ectx.Desired.HTTPAPI == nil {
					return nil
				}

				api, created, err := ensureHTTPAPI(ctx, deps.Clients, cfg, stage)
				if err != nil {
					return err
				}
				deps.HTTPAPI = api

				if created {
					if err := deps.Journal.Record(transactions.Operation{
						Type:     transactions.OpCreateAPI,
						Resource: api.APIID,
						Metadata: map[string]string{"apiId": api.APIID},
					}); err != nil {
						return err
					}
				}

				ectx.Metadata["httpApi:id"] = api.APIID
				ectx.Metadata["httpApi:endpoint"] = api.APIEndpoint
				ectx.Metadata["httpApi:stage"] = api.StageName
				return nil
			},
		},
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointEnsureLambdas,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				for fnName, fn := range cfg.Functions {
					artifact := ectx.Artifacts[fnName]
					functionPhysicalName := functionName(cfg, stage, fnName)

					var actualFunctionPtr *planner.ActualFunction
					if actualFunction, ok := deps.ActualMap[functionPhysicalName]; ok {
						actualFunctionPtr = &actualFunction
					}

					desiredFunction := planner.DesiredFunction{
						Name:            functionPhysicalName,
						Runtime:         fn.Runtime,
						Handler:         fn.Handler,
						Memory:          defaultInt(fn.Memory, 128),
						Timeout:         defaultInt(fn.Timeout, 10),
						CodeSHA256:      artifact.SHA256,
						ConfigSignature: artifact.ConfigSignature,
					}

					changeSet := detectFunctionChange(fnName, artifact, desiredFunction, actualFunctionPtr, deps.ReceiptMap)

					roleARN, roleCreated, err := ensureLambdaExecutionRole(ctx, deps.Clients, cfg, stage, fnName)
					if err != nil {
						return err
					}
					if roleCreated {
						if err := deps.Journal.Record(transactions.Operation{
							Type:     transactions.OpCreateRole,
							Resource: roleName(cfg, stage, fnName),
							Metadata: map[string]string{"roleName": roleName(cfg, stage, fnName)},
						}); err != nil {
							return err
						}
					}

					deployed, err := upsertLambdaFunction(ctx, deps.Clients, cfg, stage, fnName, fn, artifact, roleARN, changeSet)
					if err != nil {
						return err
					}
					if deployed.Created {
						if err := deps.Journal.Record(transactions.Operation{
							Type:     transactions.OpCreateLambda,
							Resource: deployed.FunctionName,
							Metadata: map[string]string{"functionName": deployed.FunctionName},
						}); err != nil {
							return err
						}
					}

					ectx.Metadata["lambda:"+fnName+":name"] = deployed.FunctionName
					invokeARN := deployed.InvokeArn
					if invokeARN == "" {
						invokeARN = deployed.FunctionArn
					}
					ectx.Metadata["lambda:"+fnName+":arn"] = invokeARN
					ectx.Metadata["lambda:"+fnName+":role"] = roleName(cfg, stage, fnName)
					ectx.Metadata["lambda:"+fnName+":changeReason"] = changeSet.Reason

					switch {
					case deployed.Created:
						ectx.Metadata["lambda:"+fnName+":operation"] = "created"
					case deployed.UpdatedCode || deployed.UpdatedConfig:
						ectx.Metadata["lambda:"+fnName+":operation"] = "updated"
					default:
						ectx.Metadata["lambda:"+fnName+":operation"] = "skipped"
					}
				}
				return nil
			},
		},
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointEnsureRoutes,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				lambdaARNByFunction := make(map[string]string)
				for fnName := range cfg.Functions {
					lambdaARNByFunction[fnName] = ectx.Metadata["lambda:"+fnName+":arn"]
				}
				for fnName, fn := range cfg.Functions {
					lambdaName := ectx.Metadata["lambda:"+fnName+":name"]
					lambdaARN := ectx.Metadata["lambda:"+fnName+":arn"]

					httpEventCount := 0
					for i, ev := range fn.Events {
						if ev.HTTP == nil {
							continue
						}
						httpEventCount++

						integrationID, integrationCreated, err := ensureHTTPIntegration(ctx, deps.Clients, deps.HTTPAPI.APIID, lambdaARN)
						if err != nil {
							return err
						}
						if integrationCreated {
							if err := deps.Journal.Record(transactions.Operation{
								Type:     transactions.OpCreateIntegration,
								Resource: integrationID,
								Metadata: map[string]string{
									"apiId":         deps.HTTPAPI.APIID,
									"integrationId": integrationID,
								},
							}); err != nil {
								return err
							}
						}

						authorizerID := ""
						authType := ""
						if ev.HTTP.Authorizer != nil {
							authorizerID, err = ensureHTTPAuthorizer(ctx, deps.Clients, deps.HTTPAPI.APIID, ev.HTTP.Authorizer, lambdaARNByFunction)
							if err != nil {
								return err
							}
							if authorizerID != "" {
								authType = "CUSTOM"
							}
						}

						// Create route first so it exists when we add Lambda permission (AWS validates route exists)
						routeID, routeCreated, err := ensureHTTPRoute(
							ctx,
							deps.Clients,
							deps.HTTPAPI.APIID,
							ev.HTTP.Method,
							ev.HTTP.Path,
							integrationID,
							authorizerID,
							authType,
						)

						if err != nil {
							return err
						}
						if routeCreated {
							if err := deps.Journal.Record(transactions.Operation{
								Type:     transactions.OpCreateRoute,
								Resource: routeID,
								Metadata: map[string]string{
									"apiId":   deps.HTTPAPI.APIID,
									"routeId": routeID,
								},
							}); err != nil {
								return err
							}
						}

						// Add Lambda permission after route exists (AWS validates route has this integration)
						if err := ensureAPIGatewayInvokePermission(
							ctx, deps.Clients, cfg, stage, deps.HTTPAPI.StageName, fnName,
							ev.HTTP.Method, ev.HTTP.Path, deps.HTTPAPI.APIID, lambdaName,
						); err != nil {
							return err
						}

						url := routeInvokeURL(deps.HTTPAPI, ev.HTTP.Path)
						key := fmt.Sprintf("%s[%d]", fnName, i)
						ectx.Outputs[key] = url
						ectx.Metadata["route:"+key+":id"] = routeID
						ectx.Metadata["route:"+key+":integrationId"] = integrationID
						ectx.Metadata["route:"+key+":url"] = url
					}

					if httpEventCount == 0 {
						url, created, err := ensureFunctionURL(ctx, deps.Clients, lambdaName)
						if err != nil {
							return err
						}
						if created {
							if err := deps.Journal.Record(transactions.Operation{
								Type:     transactions.OpCreateFunctionURL,
								Resource: lambdaName,
								Metadata: map[string]string{"functionName": lambdaName},
							}); err != nil {
								return err
							}
						}
						ectx.Outputs[fnName] = url
						ectx.Metadata["lambda:"+fnName+":url"] = url
					}
				}
				return nil
			},
		},
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointEnsureTriggers,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				lambdaNameByFunction := make(map[string]string)
				lambdaARNByFunction := make(map[string]string)
				for fnName := range cfg.Functions {
					lambdaNameByFunction[fnName] = ectx.Metadata["lambda:"+fnName+":name"]
					lambdaARNByFunction[fnName] = ectx.Metadata["lambda:"+fnName+":arn"]
				}
				cronClients := &awstriggers.CronClients{
					EventBridge: deps.Clients.EventBridge,
					Lambda:      deps.Clients.Lambda,
					Region:      deps.Clients.AWS.Region,
					AccountID:   deps.Clients.AccountID,
				}
				if err := awstriggers.EnsureCronRules(ctx, cronClients, lambdaNameByFunction, lambdaARNByFunction, cfg, stage); err != nil {
					return err
				}
				queueClients := &awstriggers.QueueClients{SQS: deps.Clients.SQS, Lambda: deps.Clients.Lambda}
				if err := awstriggers.EnsureQueueTriggers(ctx, queueClients, lambdaNameByFunction, lambdaARNByFunction, cfg, stage); err != nil {
					return err
				}
				storageClients := &awstriggers.StorageClients{S3: deps.Clients.S3, Lambda: deps.Clients.Lambda}
				if err := awstriggers.EnsureStorageTriggers(ctx, storageClients, lambdaARNByFunction, cfg, stage); err != nil {
					return err
				}
				ebClients := &awstriggers.EventBridgeClients{
					EventBridge: deps.Clients.EventBridge,
					Lambda:      deps.Clients.Lambda,
					Region:      deps.Clients.AWS.Region,
					AccountID:   deps.Clients.AccountID,
				}
				if err := awstriggers.EnsureEventBridgeRules(ctx, ebClients, lambdaARNByFunction, cfg, stage); err != nil {
					return err
				}
				return nil
			},
		},
		deployexec.PhaseFunc{
			PhaseName: deployexec.CheckpointReconcileStale,
			Fn: func(ctx context.Context, ectx *deployexec.Context) error {
				if ectx.Actual != nil && ectx.Actual.HTTPAPI != nil && ectx.Desired != nil && ectx.Desired.HTTPAPI != nil {
					if err := deleteStaleRoutes(ctx, deps.Clients, ectx.Actual.HTTPAPI.ID, ectx.Desired, ectx.Actual); err != nil {
						return err
					}
				}
				if ectx.Actual != nil && ectx.Desired != nil {
					if err := deleteStaleLambdas(ctx, deps.Clients, cfg, stage, ectx.Desired, ectx.Actual); err != nil {
						return err
					}
				}
				return nil
			},
		},
	}

	return &deployexec.Engine{
		Phases:  phases,
		Journal: deps.Journal,
	}
}

func newResumeDependencies(
	ctx context.Context,
	root string,
	cfg *config.Config,
	stage string,
	journal *transactions.Journal,
) (*ResumeDependencies, error) {
	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeDeployFailed, "load aws config failed", err)
	}

	receipt, _ := state.Load(root, stage)
	receiptMap := buildReceiptFunctionMap(receipt)

	return &ResumeDependencies{
		Clients:    clients,
		Journal:    journal,
		Receipt:    receipt,
		ReceiptMap: receiptMap,
		ActualMap:  map[string]planner.ActualFunction{},
	}, nil
}
