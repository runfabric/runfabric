package triggers

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/runfabric/runfabric/engine/internal/config"
)

// QueueClients holds SDK clients for SQS event source mappings.
type QueueClients struct {
	SQS    *sqs.Client
	Lambda *lambda.Client
}

// EnsureQueueTriggers creates or updates Lambda event source mappings for SQS queues.
func EnsureQueueTriggers(
	ctx context.Context,
	clients *QueueClients,
	lambdaNameByFunction map[string]string,
	lambdaARNByFunction map[string]string,
	cfg *config.Config,
	stage string,
) error {
	if clients == nil || clients.SQS == nil || clients.Lambda == nil {
		return nil
	}
	for fnName, fn := range cfg.Functions {
		lambdaName := lambdaNameByFunction[fnName]
		lambdaARN := lambdaARNByFunction[fnName]
		if lambdaName == "" || lambdaARN == "" {
			continue
		}
		for _, ev := range fn.Events {
			if ev.Queue == nil || ev.Queue.Queue == "" {
				continue
			}
			queueRef := ev.Queue.Queue
			batchSize := int32(10)
			if ev.Queue.Batch > 0 {
				batchSize = int32(ev.Queue.Batch)
			}
			enabled := true
			if ev.Queue.Enabled != nil {
				enabled = *ev.Queue.Enabled
			}

			queueURL := queueRef
			var queueARN string
			if strings.HasPrefix(queueRef, "https://") || strings.HasPrefix(queueRef, "arn:") {
				if strings.HasPrefix(queueRef, "arn:") {
					queueARN = queueRef
					urlOut, err := clients.SQS.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
						QueueName: strPtr(queueNameFromARN(queueRef)),
					})
					if err == nil && urlOut.QueueUrl != nil {
						queueURL = *urlOut.QueueUrl
					}
				} else {
					queueURL = queueRef
				}
			} else {
				urlOut, err := clients.SQS.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
					QueueName: &queueRef,
				})
				if err != nil {
					return fmt.Errorf("get queue url for %q: %w", queueRef, err)
				}
				if urlOut.QueueUrl == nil {
					return fmt.Errorf("queue %q has no URL", queueRef)
				}
				queueURL = *urlOut.QueueUrl
			}
			if queueARN == "" {
				attrOut, err := clients.SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl:       &queueURL,
					AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
				})
				if err != nil {
					return fmt.Errorf("get queue attributes for %q: %w", queueRef, err)
				}
				if a, ok := attrOut.Attributes["QueueArn"]; ok {
					queueARN = a
				}
			}
			if queueARN == "" {
				return fmt.Errorf("queue %q: could not resolve ARN", queueRef)
			}

			_, err := clients.Lambda.CreateEventSourceMapping(ctx, &lambda.CreateEventSourceMappingInput{
				EventSourceArn: &queueARN,
				FunctionName:   &lambdaName,
				BatchSize:      &batchSize,
				Enabled:        &enabled,
				FunctionResponseTypes: []lambdatypes.FunctionResponseType{
					lambdatypes.FunctionResponseTypeReportBatchItemFailures,
				},
			})
			if err != nil {
				if !isEventSourceMappingExists(err) {
					return fmt.Errorf("create event source mapping %q -> %q: %w", queueARN, lambdaName, err)
				}
			}
		}
	}
	return nil
}

func queueNameFromARN(arn string) string {
	// arn:aws:sqs:region:account:queuename
	parts := strings.Split(arn, ":")
	if len(parts) >= 6 {
		return parts[len(parts)-1]
	}
	return arn
}

func isEventSourceMappingExists(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ResourceConflictException") ||
		strings.Contains(err.Error(), "already exists")
}
