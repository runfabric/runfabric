package aws

import "testing"

func TestBuildHTTPStageUpdateInput(t *testing.T) {
	in := buildHTTPStageUpdateInput("api-123", "dev")
	if in == nil || in.ApiId == nil || in.StageName == nil {
		t.Fatalf("expected non-nil update input with api/stage")
	}
	if *in.ApiId != "api-123" {
		t.Fatalf("unexpected api id: %q", *in.ApiId)
	}
	if *in.StageName != "dev" {
		t.Fatalf("unexpected stage name: %q", *in.StageName)
	}
	if in.AccessLogSettings != nil {
		t.Fatalf("access log settings should not be set by default")
	}
}

func TestStageAccessLogHelpers(t *testing.T) {
	name := stageAccessLogGroupName("api123", "prod")
	if name != "/aws/apigateway/runfabric/api123/prod" {
		t.Fatalf("unexpected log group name: %q", name)
	}

	arn := stageAccessLogGroupARN("us-east-1", "123456789012", name)
	wantLogARN := "arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigateway/runfabric/api123/prod"
	if arn != wantLogARN {
		t.Fatalf("unexpected log group arn: %q", arn)
	}

	stageARN := stageResourceARN("us-east-1", "api123", "prod")
	wantStageARN := "arn:aws:apigateway:us-east-1::/apis/api123/stages/prod"
	if stageARN != wantStageARN {
		t.Fatalf("unexpected stage arn: %q", stageARN)
	}
}
