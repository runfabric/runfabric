package states

import sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"

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
	}
}
