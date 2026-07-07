package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	cloudwatchlogsv2 "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// logsLimit caps how many recent log events are returned per function.
const logsLimit int32 = 200

// Logs reads recent CloudWatch Logs events for a function's log group
// (/aws/lambda/<service>-<stage>-<function>). With no function it aggregates
// every configured function, prefixing each line with "[fn]". A missing log
// group (function never invoked) yields no lines rather than an error.
func (p *Provider) Logs(ctx context.Context, req sdkprovider.LogsRequest) (*sdkprovider.LogsResult, error) {
	cfg := req.Config
	stage := req.Stage
	service := sdkprovider.Service(cfg)
	region := sdkprovider.ProviderRegion(cfg)
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}
	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	var lines []string
	if fn := strings.TrimSpace(req.Function); fn != "" {
		got, err := fetchFunctionLogs(ctx, clients, service, stage, fn, "")
		if err != nil {
			return nil, err
		}
		lines = got
	} else {
		names := make([]string, 0)
		for name := range sdkprovider.Functions(cfg) {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			got, err := fetchFunctionLogs(ctx, clients, service, stage, name, "["+name+"] ")
			if err != nil {
				return nil, err
			}
			lines = append(lines, got...)
		}
	}

	return &sdkprovider.LogsResult{
		Provider: p.Name(),
		Function: req.Function,
		Lines:    lines,
	}, nil
}

func fetchFunctionLogs(ctx context.Context, clients *AWSClients, service, stage, fn, prefix string) ([]string, error) {
	logGroup := "/aws/lambda/" + fmt.Sprintf("%s-%s-%s", service, stage, fn)
	out, err := clients.Logs.FilterLogEvents(ctx, &cloudwatchlogsv2.FilterLogEventsInput{
		LogGroupName: awssdk.String(logGroup),
		Limit:        awssdk.Int32(logsLimit),
	})
	if err != nil {
		if isLogsNotFound(err) {
			return nil, nil // no invocations yet — no log group
		}
		return nil, fmt.Errorf("filter log events %s: %w", logGroup, err)
	}
	lines := make([]string, 0, len(out.Events))
	for _, e := range out.Events {
		lines = append(lines, prefix+strings.TrimRight(awssdk.ToString(e.Message), "\n"))
	}
	return lines, nil
}
