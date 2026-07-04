package aws

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads. The
// daemon maps the Header values onto the process env per request; the doctor
// checks Required ones. AWS also honors the SDK default chain (profiles, SSO,
// instance roles), so ambient credentials work without these being set.
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "AWS_ACCESS_KEY_ID", Header: "X-Provider-Aws-Access-Key-Id", Required: true},
	{EnvKey: "AWS_SECRET_ACCESS_KEY", Header: "X-Provider-Aws-Secret-Access-Key", Required: true},
	{EnvKey: "AWS_SESSION_TOKEN", Header: "X-Provider-Aws-Session-Token"},
	{EnvKey: "AWS_REGION", Header: "X-Provider-Aws-Region", Required: true, Mirror: "AWS_DEFAULT_REGION", Placeholder: "us-east-1"},
}
