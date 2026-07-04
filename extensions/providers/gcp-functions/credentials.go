package gcp

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// CredentialVars declares the credential env vars this provider reads.
// Auth is either GCP_ACCESS_TOKEN or a GOOGLE_APPLICATION_CREDENTIALS
// service-account key (exchanged + auto-refreshed via plugin-sdk gcpauth), so
// neither is individually Required; a project is a hard requirement (either
// GCP_PROJECT or GCP_PROJECT_ID spelling).
var CredentialVars = []sdkprovider.CredentialVar{
	{EnvKey: "GCP_ACCESS_TOKEN", Header: "X-Provider-Gcp-Access-Token"},
	{EnvKey: "GOOGLE_APPLICATION_CREDENTIALS", Placeholder: "/path/to/service-account.json"},
	{EnvKey: "GCP_PROJECT", Header: "X-Provider-Gcp-Project", Required: true, Mirror: "GCP_PROJECT_ID"},
	{EnvKey: "GCP_UPLOAD_BUCKET", Header: "X-Provider-Gcp-Upload-Bucket"},
	{EnvKey: "GCP_REGION", Header: "X-Provider-Gcp-Region", Placeholder: "us-central1"},
}
