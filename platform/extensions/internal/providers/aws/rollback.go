package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/platform/core/state/transactions"
)

type awsRollbacker struct {
	clients *AWSClients
}

func newAWSRollbacker(clients *AWSClients) *awsRollbacker {
	return &awsRollbacker{clients: clients}
}

func (r *awsRollbacker) Rollback(ctx context.Context, op transactions.Operation) error {
	switch op.Type {
	case transactions.OpCreateRoute:
		apiID := op.Metadata["apiId"]
		routeID := op.Metadata["routeId"]
		if apiID == "" || routeID == "" {
			return nil
		}
		_, err := r.clients.APIGW.DeleteRoute(ctx, &apigatewayv2.DeleteRouteInput{
			ApiId:   &apiID,
			RouteId: &routeID,
		})
		return err

	case transactions.OpCreateIntegration:
		apiID := op.Metadata["apiId"]
		integrationID := op.Metadata["integrationId"]
		if apiID == "" || integrationID == "" {
			return nil
		}
		_, err := r.clients.APIGW.DeleteIntegration(ctx, &apigatewayv2.DeleteIntegrationInput{
			ApiId:         &apiID,
			IntegrationId: &integrationID,
		})
		return err

	case transactions.OpCreateFunctionURL:
		functionName := op.Metadata["functionName"]
		if functionName == "" {
			return nil
		}
		return deleteFunctionURL(ctx, r.clients, functionName)

	case transactions.OpCreateLambda:
		functionName := op.Metadata["functionName"]
		if functionName == "" {
			return nil
		}

		_, err := r.clients.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
			FunctionName: &functionName,
		})

		if err != nil {
			return fmt.Errorf("delete lambda %s: %w", functionName, err)
		}

		return err

	case transactions.OpCreateRole:
		roleName := op.Metadata["roleName"]
		if roleName == "" {
			return nil
		}

		attached, err := r.clients.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: &roleName,
		})
		if err == nil {
			for _, p := range attached.AttachedPolicies {
				if p.PolicyArn == nil {
					continue
				}
				_, _ = r.clients.IAM.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
					RoleName:  &roleName,
					PolicyArn: p.PolicyArn,
				})
			}
		}

		_, err = r.clients.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
			RoleName: &roleName,
		})
		return err

	case transactions.OpCreateAPI:
		apiID := op.Metadata["apiId"]
		if apiID == "" {
			return nil
		}
		_, err := r.clients.APIGW.DeleteApi(ctx, &apigatewayv2.DeleteApiInput{
			ApiId: &apiID,
		})
		return err

	default:
		return fmt.Errorf("unsupported rollback operation: %s", op.Type)
	}
}
