package states

import (
	sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

// BuiltinStateManifests returns state plugin metadata entries exposed in
// built-in extension catalogs.
func BuiltinStateManifests() []sdkrouter.PluginMeta {
	return []sdkrouter.PluginMeta{
		{
			ID:          "local",
			Name:        "Local State Backend",
			Description: "Stores deployment state in local files under .runfabric/",
		},
		{
			ID:          "sqlite",
			Name:        "SQLite State Backend",
			Description: "Stores deployment receipts in SQLite with local journals/locks",
		},
		{
			ID:          "postgres",
			Name:        "Postgres State Backend",
			Description: "Stores deployment receipts in Postgres with local journals/locks",
		},
		{
			ID:          "dynamodb",
			Name:        "DynamoDB State Backend",
			Description: "Stores deployment receipts in DynamoDB with local journals/locks",
		},
		{
			ID:          "s3",
			Name:        "S3 State Backend",
			Description: "Stores deployment receipts in S3 with local journals/locks",
		},
		{
			ID:          "gcs",
			Name:        "GCS State Backend",
			Description: "Stores deployment receipts in Google Cloud Storage with local journals/locks",
		},
		{
			ID:          "azblob",
			Name:        "Azure Blob State Backend",
			Description: "Stores deployment receipts in Azure Blob Storage with local journals/locks",
		},
	}
}

// BuiltinStateCredentials declares the env vars each built-in state backend
// reads, keyed by backend kind — the state-side counterpart of each provider's
// CredentialVars.
//
// Header (X-State-*) marks secrets a daemon accepts per request, so a tenant
// can bring their own state store. Vars whose values ride the manifest
// (bucket/container names) or that another group already carries (AWS chain
// via X-Provider-Aws-*, GCP token via X-Provider-Gcp-Access-Token) stay
// env-only.
//
// A yaml `backend:` field can stand in for the env var (e.g. backend.s3Bucket
// for RUNFABRIC_S3_BUCKET); Required means "the backend cannot start when
// neither the env var nor its config field is set". local and sqlite need no
// credentials and declare none.
func BuiltinStateCredentials() map[string][]sdkprovider.CredentialVar {
	return map[string][]sdkprovider.CredentialVar{
		"postgres": {
			// backend.postgresConnectionStringEnv may name an ADDITIONAL env
			// alias per project, but this default key always works (the
			// per-request header targets it), so it can never be renamed away.
			{EnvKey: "RUNFABRIC_STATE_POSTGRES_URL", Header: "X-State-Postgres-Url", Required: true, Placeholder: "postgres://user:pass@localhost:5432/runfabric"},
		},
		"s3": {
			{EnvKey: "RUNFABRIC_S3_BUCKET", Required: true},
			{EnvKey: "RUNFABRIC_S3_PREFIX"},
			{EnvKey: "RUNFABRIC_DYNAMODB_TABLE"},
		},
		"dynamodb": {
			{EnvKey: "RUNFABRIC_DYNAMODB_TABLE", Required: true},
		},
		"gcs": {
			// Auth: GCP_ACCESS_TOKEN or a GOOGLE_APPLICATION_CREDENTIALS
			// service-account key — either works, so neither is Required.
			{EnvKey: "GCP_ACCESS_TOKEN"},
			{EnvKey: "GOOGLE_APPLICATION_CREDENTIALS", Placeholder: "/path/to/service-account.json"},
			{EnvKey: "RUNFABRIC_GCS_BUCKET", Required: true},
			{EnvKey: "RUNFABRIC_GCS_PREFIX", Placeholder: "runfabric/dev"},
		},
		"azblob": {
			// Auth: connection string OR account+key — either pair works, so
			// none is individually Required; the backend errors clearly when
			// neither is set.
			{EnvKey: "RUNFABRIC_AZBLOB_CONTAINER", Required: true},
			{EnvKey: "RUNFABRIC_AZBLOB_PREFIX", Placeholder: "runfabric/dev"},
			{EnvKey: "AZURE_STORAGE_CONNECTION_STRING", Header: "X-State-Azblob-Connection-String"},
			{EnvKey: "AZURE_STORAGE_ACCOUNT", Header: "X-State-Azblob-Account"},
			{EnvKey: "AZURE_STORAGE_KEY", Header: "X-State-Azblob-Key"},
		},
	}
}
