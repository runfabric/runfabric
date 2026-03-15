package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/runfabric/runfabric/internal/config"
	appErrs "github.com/runfabric/runfabric/internal/errors"
	"github.com/runfabric/runfabric/internal/providers"

	cloudwatchlogsv2 "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

func (p *Provider) Logs(cfg *config.Config, stage, function string) (*providers.LogsResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clients, err := loadClients(ctx, cfg.Provider.Region)
	if err != nil {
		return nil, appErrs.Wrap(appErrs.CodeLogsFailed, "load aws config failed", err)
	}

	logGroup := fmt.Sprintf("/aws/lambda/%s", functionName(cfg, stage, function))

	streamsOut, err := clients.Logs.DescribeLogStreams(ctx, &cloudwatchlogsv2.DescribeLogStreamsInput{
		LogGroupName: &logGroup,
		OrderBy:      "LastEventTime",
		Descending:   boolPtr(true),
		Limit:        int32PtrValue(5),
	})
	if err != nil {
		if isLogsNotFound(err) {
			return &providers.LogsResult{
				Provider: p.Name(),
				Function: function,
				Lines:    []string{"log group not found yet"},
			}, nil
		}
		return nil, appErrs.Wrap(appErrs.CodeLogsFailed, "describe log streams failed", err)
	}

	lines := []string{}

	for _, stream := range streamsOut.LogStreams {
		if stream.LogStreamName == nil {
			continue
		}

		eventsOut, err := clients.Logs.GetLogEvents(ctx, &cloudwatchlogsv2.GetLogEventsInput{
			LogGroupName:  &logGroup,
			LogStreamName: stream.LogStreamName,
			Limit:         int32PtrValue(50),
			StartFromHead: boolPtr(false),
		})
		if err != nil {
			continue
		}

		for _, ev := range eventsOut.Events {
			msg := ""
			if ev.Message != nil {
				msg = *ev.Message
			}
			lines = append(lines, msg)
		}
	}

	sort.Strings(lines)

	if len(lines) == 0 {
		lines = append(lines, "no log events found")
	}

	return &providers.LogsResult{
		Provider: p.Name(),
		Function: function,
		Lines:    lines,
	}, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func int32PtrValue(v int32) *int32 {
	return &v
}
