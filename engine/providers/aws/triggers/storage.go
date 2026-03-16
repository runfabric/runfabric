package triggers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/runfabric/runfabric/engine/internal/config"
)

// StorageClients holds SDK clients for S3 bucket notifications and Lambda permission.
type StorageClients struct {
	S3     *s3.Client
	Lambda *lambda.Client
}

// storageNotification describes one S3 notification to attach (grouped by bucket for batching).
type storageNotification struct {
	bucket    string
	lambdaARN string
	events    []string
	prefix    string
	suffix    string
	id        string
	fnName    string
	eventIdx  int
}

// EnsureStorageTriggers configures S3 bucket notifications to invoke Lambda on object events.
// Groups work by bucket so each bucket is read and written once instead of once per event.
func EnsureStorageTriggers(
	ctx context.Context,
	clients *StorageClients,
	lambdaARNByFunction map[string]string,
	cfg *config.Config,
	stage string,
) error {
	if clients == nil || clients.S3 == nil || clients.Lambda == nil {
		return nil
	}

	// Collect all storage events grouped by bucket (one Get + one Put per bucket).
	byBucket := make(map[string][]storageNotification)
	for fnName, fn := range cfg.Functions {
		lambdaARN := lambdaARNByFunction[fnName]
		if lambdaARN == "" {
			continue
		}
		for i, ev := range fn.Events {
			if ev.Storage == nil || ev.Storage.Bucket == "" {
				continue
			}
			bucket := ev.Storage.Bucket
			events := ev.Storage.Events
			if len(events) == 0 {
				events = []string{"s3:ObjectCreated:*"}
			}
			id := fmt.Sprintf("runfabric-%s-%s-%s-%d", cfg.Service, stage, fnName, i)
			byBucket[bucket] = append(byBucket[bucket], storageNotification{
				bucket:    bucket,
				lambdaARN: lambdaARN,
				events:    events,
				prefix:    ev.Storage.Prefix,
				suffix:    ev.Storage.Suffix,
				id:        id,
				fnName:    fnName,
				eventIdx:  i,
			})
		}
	}

	for bucket, notifs := range byBucket {
		cfgOut, err := clients.S3.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
			Bucket: &bucket,
		})
		if err != nil {
			return fmt.Errorf("get bucket notification %q: %w", bucket, err)
		}

		lambdaConfigs := append([]s3types.LambdaFunctionConfiguration{}, cfgOut.LambdaFunctionConfigurations...)
		for _, n := range notifs {
			found := false
			for j := range lambdaConfigs {
				if lambdaConfigs[j].Id != nil && *lambdaConfigs[j].Id == n.id {
					lambdaConfigs[j].LambdaFunctionArn = &n.lambdaARN
					lambdaConfigs[j].Events = make([]s3types.Event, len(n.events))
					for k, e := range n.events {
						lambdaConfigs[j].Events[k] = s3types.Event(e)
					}
					lambdaConfigs[j].Filter = bucketFilter(n.prefix, n.suffix)
					found = true
					break
				}
			}
			if !found {
				s3Events := make([]s3types.Event, 0, len(n.events))
				for _, e := range n.events {
					s3Events = append(s3Events, s3types.Event(e))
				}
				id := n.id
				lambdaConfigs = append(lambdaConfigs, s3types.LambdaFunctionConfiguration{
					Id:                &id,
					LambdaFunctionArn: &n.lambdaARN,
					Events:            s3Events,
					Filter:            bucketFilter(n.prefix, n.suffix),
				})
			}
		}

		_, err = clients.S3.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
			Bucket: &bucket,
			NotificationConfiguration: &s3types.NotificationConfiguration{
				LambdaFunctionConfigurations: lambdaConfigs,
				TopicConfigurations:          cfgOut.TopicConfigurations,
				QueueConfigurations:          cfgOut.QueueConfigurations,
				EventBridgeConfiguration:     cfgOut.EventBridgeConfiguration,
			},
		})
		if err != nil {
			return fmt.Errorf("put bucket notification %q: %w", bucket, err)
		}
	}

	// Lambda AddPermission is per-event (no batching).
	for _, notifs := range byBucket {
		for _, n := range notifs {
			stmtID := fmt.Sprintf("runfabric-s3-%s-%s-%d", n.fnName, stage, n.eventIdx)
			_, err := clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
				FunctionName: strPtr(extractLambdaNameFromARN(n.lambdaARN)),
				StatementId:  &stmtID,
				Action:       strPtr("lambda:InvokeFunction"),
				Principal:    strPtr("s3.amazonaws.com"),
				SourceArn:    strPtr(fmt.Sprintf("arn:aws:s3:::%s", n.bucket)),
			})
			if err != nil && !isPermissionAlreadyExists(err) {
				return fmt.Errorf("add s3 invoke permission for %q: %w", n.fnName, err)
			}
		}
	}
	return nil
}

func bucketFilter(prefix, suffix string) *s3types.NotificationConfigurationFilter {
	if prefix == "" && suffix == "" {
		return nil
	}
	f := &s3types.NotificationConfigurationFilter{
		Key: &s3types.S3KeyFilter{
			FilterRules: []s3types.FilterRule{},
		},
	}
	if prefix != "" {
		f.Key.FilterRules = append(f.Key.FilterRules, s3types.FilterRule{Name: s3types.FilterRuleNamePrefix, Value: &prefix})
	}
	if suffix != "" {
		f.Key.FilterRules = append(f.Key.FilterRules, s3types.FilterRule{Name: s3types.FilterRuleNameSuffix, Value: &suffix})
	}
	return f
}

func extractLambdaNameFromARN(arn string) string {
	// arn:aws:lambda:region:account:function:name
	for i := len(arn) - 1; i >= 0; i-- {
		if arn[i] == ':' {
			return arn[i+1:]
		}
	}
	return arn
}
