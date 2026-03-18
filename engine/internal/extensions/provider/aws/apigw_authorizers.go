package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/runfabric/runfabric/engine/internal/config"
)

// ensureHTTPAuthorizer creates or resolves an authorizer and returns its ID.
// For lambda type, lambdaARNByFunction must contain the authorizer function ARN (key auth.Function).
func ensureHTTPAuthorizer(ctx context.Context, clients *AWSClients, apiID string, auth *config.AuthorizerConfig, lambdaARNByFunction map[string]string) (string, error) {
	if auth == nil {
		return "", nil
	}

	switch auth.Type {
	case "jwt":
		in := &apigatewayv2.CreateAuthorizerInput{
			ApiId:          &apiID,
			Name:           &auth.Name,
			AuthorizerType: apigwtypes.AuthorizerTypeJwt,
			IdentitySource: auth.IdentitySources,
			JwtConfiguration: &apigwtypes.JWTConfiguration{
				Audience: auth.Audience,
				Issuer:   &auth.Issuer,
			},
		}
		out, err := clients.APIGW.CreateAuthorizer(ctx, in)
		if err != nil {
			return "", err
		}
		if out.AuthorizerId == nil {
			return "", fmt.Errorf("missing authorizer id")
		}
		return *out.AuthorizerId, nil

	case "iam":
		return "", nil

	case "lambda":
		if auth.Function == "" {
			return "", fmt.Errorf("lambda authorizer requires authorizer.function (function name)")
		}
		lambdaARN, ok := lambdaARNByFunction[auth.Function]
		if !ok || lambdaARN == "" {
			return "", fmt.Errorf("lambda authorizer function %q not found in deployed Lambdas", auth.Function)
		}
		// AuthorizerUri: arn:aws:apigateway:{region}:lambda:path/2015-03-31/functions/{lambdaArn}/invocations
		region := clients.AWS.Region
		authorizerURI := fmt.Sprintf("arn:aws:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations", region, lambdaARN)
		name := auth.Name
		if name == "" {
			name = "lambda-" + auth.Function
		}
		identitySources := auth.IdentitySources
		if len(identitySources) == 0 {
			identitySources = []string{"$request.header.Authorization"}
		}
		in := &apigatewayv2.CreateAuthorizerInput{
			ApiId:                          &apiID,
			Name:                           &name,
			AuthorizerType:                 apigwtypes.AuthorizerTypeRequest,
			IdentitySource:                 identitySources,
			AuthorizerUri:                  &authorizerURI,
			AuthorizerPayloadFormatVersion: str("2.0"),
			EnableSimpleResponses:          boolPtr(true),
		}
		out, err := clients.APIGW.CreateAuthorizer(ctx, in)
		if err != nil {
			return "", fmt.Errorf("create lambda authorizer: %w", err)
		}
		if out.AuthorizerId == nil {
			return "", fmt.Errorf("missing authorizer id")
		}
		// Allow API Gateway to invoke the authorizer Lambda.
		stmtID := "apigw-invoke-" + *out.AuthorizerId
		_, err = clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
			FunctionName: str(extractLambdaNameFromARN(lambdaARN)),
			StatementId:  &stmtID,
			Action:       str("lambda:InvokeFunction"),
			Principal:    str("apigateway.amazonaws.com"),
			SourceArn:    str(fmt.Sprintf("arn:aws:execute-api:%s:%s:%s/authorizers/%s", region, clients.AccountID, apiID, *out.AuthorizerId)),
		})
		if err != nil && !isLambdaConflict(err) {
			return "", fmt.Errorf("add apigateway invoke permission for authorizer: %w", err)
		}
		return *out.AuthorizerId, nil

	default:
		return "", fmt.Errorf("unsupported authorizer type %q", auth.Type)
	}
}

func extractLambdaNameFromARN(arn string) string {
	for i := len(arn) - 1; i >= 0; i-- {
		if arn[i] == ':' {
			return arn[i+1:]
		}
	}
	return arn
}
