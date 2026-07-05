package manifests

import (
	"strings"
	"testing"
)

func TestValidateCredentialSpecs(t *testing.T) {
	ok := []CredentialSpec{
		{EnvKey: "AWS_ACCESS_KEY_ID", Header: "X-Provider-Aws-Access-Key-Id", Required: true},
		{EnvKey: "AWS_REGION", Header: "X-Provider-Aws-Region", Mirror: "AWS_DEFAULT_REGION"},
		{EnvKey: "RUNFABRIC_S3_BUCKET"},
		{EnvKey: "RUNFABRIC_ROUTER_API_TOKEN", Fallback: "CLOUDFLARE_API_TOKEN"},
	}
	if err := ValidateCredentialSpecs(ok); err != nil {
		t.Fatalf("valid specs rejected: %v", err)
	}
	if err := ValidateCredentialSpecs(nil); err != nil {
		t.Fatalf("empty specs must be valid: %v", err)
	}

	cases := []struct {
		name    string
		specs   []CredentialSpec
		wantErr string
	}{
		{"missing envKey", []CredentialSpec{{Header: "X-A-B"}}, "envKey is required"},
		{"lowercase envKey", []CredentialSpec{{EnvKey: "aws_key"}}, "must match"},
		{"envKey with dash", []CredentialSpec{{EnvKey: "AWS-KEY"}}, "must match"},
		{"duplicate envKey", []CredentialSpec{{EnvKey: "A_B"}, {EnvKey: "A_B"}}, "duplicate envKey"},
		{"header without X-", []CredentialSpec{{EnvKey: "A_B", Header: "Provider-Token"}}, "must match"},
		{"header with space", []CredentialSpec{{EnvKey: "A_B", Header: "X-Bad Header"}}, "must match"},
		{"duplicate header", []CredentialSpec{
			{EnvKey: "A_B", Header: "X-Same"},
			{EnvKey: "C_D", Header: "X-Same"},
		}, "duplicate header"},
		{"bad mirror", []CredentialSpec{{EnvKey: "A_B", Mirror: "bad-mirror"}}, "must match"},
		{"self mirror", []CredentialSpec{{EnvKey: "A_B", Mirror: "A_B"}}, "differ from envKey"},
		{"mirror collides with envKey", []CredentialSpec{
			{EnvKey: "A_B", Mirror: "C_D"},
			{EnvKey: "C_D"},
		}, "collides with a declared envKey"},
		{"bad fallback", []CredentialSpec{{EnvKey: "A_B", Fallback: "bad-key"}}, "must match"},
		{"self fallback", []CredentialSpec{{EnvKey: "A_B", Fallback: "A_B"}}, "differ from envKey"},
	}
	for _, tc := range cases {
		err := ValidateCredentialSpecs(tc.specs)
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: err = %v, want containing %q", tc.name, err, tc.wantErr)
		}
	}
}
