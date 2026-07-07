package dynamodb

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares the backend: config block this state backend contributes to
// `runfabric init` (co-located with the backend, like a provider's Scaffold).
var Scaffold = []sdkprovider.ScaffoldConfigLine{
	{Key: "lockTable", Value: "${env:RUNFABRIC_DYNAMODB_TABLE}"},
}
