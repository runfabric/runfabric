package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
)

func buildHTTPStageUpdateInput(apiID, stage string) *apigatewayv2.UpdateStageInput {
	return &apigatewayv2.UpdateStageInput{
		ApiId:     &apiID,
		StageName: &stage,
	}
}

func stageAccessLogGroupName(apiID, stage string) string {
	return fmt.Sprintf("/aws/apigateway/runfabric/%s/%s", apiID, stage)
}

func stageAccessLogGroupARN(region, accountID, logGroupName string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", region, accountID, logGroupName)
}

func stageResourceARN(region, apiID, stage string) string {
	return fmt.Sprintf("arn:aws:apigateway:%s::/apis/%s/stages/%s", region, apiID, stage)
}
