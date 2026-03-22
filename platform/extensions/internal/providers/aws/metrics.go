package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/runfabric/runfabric/platform/core/model/config"
)

// LambdaMetrics holds per-function CloudWatch metrics for Lambda.
type LambdaMetrics struct {
	Invocations *float64 `json:"invocations,omitempty"`
	Errors      *float64 `json:"errors,omitempty"`
	DurationAvg *float64 `json:"durationAvgMs,omitempty"`
}

// FetchLambdaMetrics returns CloudWatch metrics (Invocations, Errors, Duration) for each function in the config, for the last hour.
func FetchLambdaMetrics(ctx context.Context, cfg *config.Config, stage string) (map[string]LambdaMetrics, error) {
	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, err
	}
	end := time.Now().UTC()
	start := end.Add(-1 * time.Hour)
	out := make(map[string]LambdaMetrics)
	for fnName := range cfg.Functions {
		physicalName := functionName(cfg, stage, fnName)
		m, err := getFunctionMetrics(ctx, clients, physicalName, start, end)
		if err != nil {
			continue
		}
		out[fnName] = m
	}
	return out, nil
}

func getFunctionMetrics(ctx context.Context, clients *AWSClients, functionName string, start, end time.Time) (LambdaMetrics, error) {
	dim := cloudwatchtypes.Dimension{Name: aws.String("FunctionName"), Value: aws.String(functionName)}
	namespace := "AWS/Lambda"
	period := int32(3600) // 1 hour bucket

	var invocations, errorsSum *float64
	var durationAvg *float64

	// Invocations
	invOut, err := clients.CloudWatch.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String("Invocations"),
		Dimensions: []cloudwatchtypes.Dimension{dim},
		StartTime:  aws.Time(start),
		EndTime:    aws.Time(end),
		Period:     aws.Int32(period),
		Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticSum},
	})
	if err == nil && len(invOut.Datapoints) > 0 {
		sum := invOut.Datapoints[0].Sum
		invocations = sum
	}

	// Errors
	errOut, err := clients.CloudWatch.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String("Errors"),
		Dimensions: []cloudwatchtypes.Dimension{dim},
		StartTime:  aws.Time(start),
		EndTime:    aws.Time(end),
		Period:     aws.Int32(period),
		Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticSum},
	})
	if err == nil && len(errOut.Datapoints) > 0 {
		sum := errOut.Datapoints[0].Sum
		errorsSum = sum
	}

	// Duration (average)
	durOut, err := clients.CloudWatch.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String("Duration"),
		Dimensions: []cloudwatchtypes.Dimension{dim},
		StartTime:  aws.Time(start),
		EndTime:    aws.Time(end),
		Period:     aws.Int32(period),
		Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticAverage},
	})
	if err == nil && len(durOut.Datapoints) > 0 && durOut.Datapoints[0].Average != nil {
		avg := *durOut.Datapoints[0].Average
		durationAvg = &avg
	}

	return LambdaMetrics{
		Invocations: invocations,
		Errors:      errorsSum,
		DurationAvg: durationAvg,
	}, nil
}
