package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
)

// Recover inspects the real deployed state of a stage's functions and reports
// it per recovery mode. It queries Lambda GetFunction for each configured
// function rather than returning canned acknowledgements: inspect reports the
// snapshot; resume adds the snapshot as ResumeData so the caller can continue a
// partial deploy; rollback reports the snapshot so an operator can see what
// exists (code-level rollback to a prior version requires published versions,
// which Deploy does not yet create).
func (p *Provider) Recover(ctx context.Context, req sdkprovider.RecoveryRequest) (*sdkprovider.RecoveryResult, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	statusByMode := map[string]string{"inspect": "inspected", "resume": "resumed", "rollback": "rolled_back"}
	status, ok := statusByMode[mode]
	if !ok {
		return nil, fmt.Errorf("unsupported recovery mode %q", req.Mode)
	}

	cfg := req.Config
	stage := req.Stage
	service := sdkprovider.Service(cfg)
	if service == "" {
		service = req.Service
	}
	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = sdkprovider.ProviderRegion(cfg)
	}
	if region == "" {
		region = sdkprovider.Env("AWS_REGION")
	}
	clients, err := loadClients(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("load aws clients: %w", err)
	}

	metadata := map[string]string{"service": service, "stage": stage, "region": region}
	var errs []string
	present, absent, rolledBack := 0, 0, 0

	names := make([]string, 0)
	for fn := range sdkprovider.Functions(cfg) {
		names = append(names, fn)
	}
	sort.Strings(names)
	for _, fn := range names {
		functionName := fmt.Sprintf("%s-%s-%s", service, stage, fn)
		out, err := clients.Lambda.GetFunction(ctx, &lambdav2.GetFunctionInput{
			FunctionName: awssdk.String(functionName),
		})
		if err != nil {
			if isLambdaNotFound(err) {
				absent++
				metadata[fn] = "absent"
				continue
			}
			errs = append(errs, fmt.Sprintf("%s: %v", functionName, err))
			continue
		}
		present++
		state, version, modified := "", "", ""
		if out.Configuration != nil {
			state = string(out.Configuration.State)
			version = awssdk.ToString(out.Configuration.Version)
			modified = awssdk.ToString(out.Configuration.LastModified)
		}
		metadata[fn] = fmt.Sprintf("state=%s version=%s lastModified=%s", state, version, modified)

		// Rollback repoints the `live` alias to the previous published version.
		if mode == "rollback" {
			from, to, ok, rbErr := rollbackAlias(ctx, clients, functionName)
			switch {
			case rbErr != nil:
				errs = append(errs, fmt.Sprintf("%s: %v", functionName, rbErr))
			case ok:
				metadata["rollback:"+fn] = fmt.Sprintf("%s->%s", from, to)
				rolledBack++
			default:
				metadata["rollback:"+fn] = "no earlier version"
			}
		}
	}

	result := &sdkprovider.RecoveryResult{
		Recovered: len(errs) == 0,
		Mode:      mode,
		Status:    status,
		Message:   fmt.Sprintf("aws recovery %s: %d function(s) present, %d absent", mode, present, absent),
		Metadata:  metadata,
		Errors:    errs,
	}
	if mode == "rollback" {
		result.Message = fmt.Sprintf("aws recovery rollback: %d function(s) reverted to a previous version (%d present, %d absent)", rolledBack, present, absent)
	}
	if mode == "resume" {
		result.ResumeData = map[string]any{"present": present, "absent": absent}
	}
	return result, nil
}
