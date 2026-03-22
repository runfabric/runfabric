package triggers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// EventBridgeClients holds SDK clients for custom EventBridge rules and Lambda permission.
type EventBridgeClients struct {
	EventBridge *eventbridge.Client
	Lambda      *lambda.Client
	Region      string
	AccountID   string
}

// EnsureEventBridgeRules creates EventBridge rules with the given event pattern and adds Lambda as target.
func EnsureEventBridgeRules(
	ctx context.Context,
	clients *EventBridgeClients,
	lambdaARNByFunction map[string]string,
	cfg *config.Config,
	stage string,
) error {
	if clients == nil || clients.EventBridge == nil || clients.Lambda == nil {
		return nil
	}
	for fnName, fn := range cfg.Functions {
		lambdaARN := lambdaARNByFunction[fnName]
		if lambdaARN == "" {
			continue
		}
		for i, ev := range fn.Events {
			if ev.EventBridge == nil {
				continue
			}
			pattern := ev.EventBridge.Pattern
			if len(pattern) == 0 {
				pattern = map[string]any{"source": []string{"runfabric"}}
			}
			patternJSON, err := json.Marshal(pattern)
			if err != nil {
				return fmt.Errorf("eventbridge pattern for %q: %w", fnName, err)
			}
			ruleName := fmt.Sprintf("%s-%s-%s-eb-%d", cfg.Service, stage, fnName, i)
			busName := ev.EventBridge.Bus
			if busName == "" {
				busName = "default"
			}

			_, err = clients.EventBridge.PutRule(ctx, &eventbridge.PutRuleInput{
				Name:         &ruleName,
				EventPattern: strPtr(string(patternJSON)),
				State:        ebtypes.RuleStateEnabled,
				EventBusName: &busName,
				Description:  strPtr(fmt.Sprintf("runfabric eventbridge for %s", fnName)),
			})
			if err != nil {
				return fmt.Errorf("put eventbridge rule %q: %w", ruleName, err)
			}

			_, err = clients.EventBridge.PutTargets(ctx, &eventbridge.PutTargetsInput{
				Rule:         &ruleName,
				EventBusName: &busName,
				Targets: []ebtypes.Target{
					{
						Id:  strPtr("1"),
						Arn: &lambdaARN,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("put eventbridge targets for %q: %w", ruleName, err)
			}

			ruleARN := ruleARNForPermission(clients.Region, clients.AccountID, busName, ruleName)
			stmtID := fmt.Sprintf("runfabric-eb-%s-%s-%d", fnName, stage, i)
			lambdaName := extractLambdaNameFromARN(lambdaARN)
			_, err = clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
				FunctionName: &lambdaName,
				StatementId:  &stmtID,
				Action:       strPtr("lambda:InvokeFunction"),
				Principal:    strPtr("events.amazonaws.com"),
				SourceArn:    &ruleARN,
			})
			if err != nil && !isPermissionAlreadyExists(err) {
				return fmt.Errorf("add eventbridge invoke permission for %q: %w", fnName, err)
			}
		}
	}
	return nil
}

func ruleARNForPermission(region, accountID, busName, ruleName string) string {
	if busName == "" || busName == "default" {
		return fmt.Sprintf("arn:aws:events:%s:%s:rule/%s", region, accountID, ruleName)
	}
	return fmt.Sprintf("arn:aws:events:%s:%s:rule/%s/%s", region, accountID, busName, ruleName)
}
