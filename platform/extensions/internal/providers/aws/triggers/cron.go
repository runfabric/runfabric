// Package triggers implements AWS trigger resources (cron, queue, storage, eventbridge).

package triggers

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// CronClients holds SDK clients needed for cron (EventBridge + Lambda permission).
type CronClients struct {
	EventBridge *eventbridge.Client
	Lambda      *lambda.Client
	Region      string
	AccountID   string
}

// EnsureCronRules creates or updates EventBridge rules for cron triggers and adds Lambda as target.
func EnsureCronRules(
	ctx context.Context,
	clients *CronClients,
	lambdaNameByFunction map[string]string,
	lambdaARNByFunction map[string]string,
	cfg *config.Config,
	stage string,
) error {
	if clients == nil || clients.EventBridge == nil || clients.Lambda == nil {
		return fmt.Errorf("cron: EventBridge and Lambda clients required")
	}
	for fnName, fn := range cfg.Functions {
		lambdaName := lambdaNameByFunction[fnName]
		lambdaARN := lambdaARNByFunction[fnName]
		if lambdaName == "" || lambdaARN == "" {
			continue
		}
		for i, ev := range fn.Events {
			if ev.Cron == "" {
				continue
			}
			scheduleExpr := normalizeScheduleExpression(ev.Cron)
			ruleName := fmt.Sprintf("%s-%s-%s-cron-%d", cfg.Service, stage, fnName, i)

			_, err := clients.EventBridge.PutRule(ctx, &eventbridge.PutRuleInput{
				Name:               &ruleName,
				ScheduleExpression: &scheduleExpr,
				State:              ebtypes.RuleStateEnabled,
				Description:        strPtr(fmt.Sprintf("runfabric cron for %s %s", fnName, ev.Cron)),
			})
			if err != nil {
				return fmt.Errorf("put eventbridge rule %q: %w", ruleName, err)
			}

			ruleARN := fmt.Sprintf("arn:aws:events:%s:%s:rule/%s", clients.Region, clients.AccountID, ruleName)
			_, err = clients.EventBridge.PutTargets(ctx, &eventbridge.PutTargetsInput{
				Rule: &ruleName,
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

			stmtID := fmt.Sprintf("runfabric-%s-%s-%d", fnName, stage, i)
			_, err = clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
				FunctionName: &lambdaName,
				StatementId:  &stmtID,
				Action:       strPtr("lambda:InvokeFunction"),
				Principal:    strPtr("events.amazonaws.com"),
				SourceArn:    &ruleARN,
			})
			if err != nil && !isPermissionAlreadyExists(err) {
				return fmt.Errorf("add eventbridge invoke permission for %q: %w", lambdaName, err)
			}
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }

func normalizeScheduleExpression(cron string) string {
	cron = strings.TrimSpace(cron)
	if cron == "" {
		return "rate(5 minutes)"
	}
	if strings.HasPrefix(cron, "rate(") || strings.HasPrefix(cron, "cron(") {
		return cron
	}
	// Unix 5-field: min hour dom month dow. AWS wants 6: min hour dom month dow year; use ? for dom/dow when *.
	parts := strings.Fields(cron)
	if len(parts) == 5 {
		return "cron(" + parts[0] + " " + parts[1] + " " + parts[2] + " " + parts[3] + " " + parts[4] + " ? *)"
	}
	return "cron(" + cron + ")"
}

func isPermissionAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ResourceConflictException") ||
		strings.Contains(err.Error(), "already exists")
}
