package aws

import (
	"errors"

	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func isLambdaNotFound(err error) bool {
	var e *lambdatypes.ResourceNotFoundException
	return errors.As(err, &e)
}

func isLambdaConflict(err error) bool {
	var e *lambdatypes.ResourceConflictException
	return errors.As(err, &e)
}

func isIAMNoSuchEntity(err error) bool {
	var e *iamtypes.NoSuchEntityException
	return errors.As(err, &e)
}

func isLogsNotFound(err error) bool {
	var e *cloudwatchlogstypes.ResourceNotFoundException
	return errors.As(err, &e)
}

func isLogsAlreadyExists(err error) bool {
	var e *cloudwatchlogstypes.ResourceAlreadyExistsException
	return errors.As(err, &e)
}
