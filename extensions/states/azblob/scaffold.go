package azblob

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares the backend: config block this state backend contributes to
// `runfabric init` (co-located with the backend, like a provider's Scaffold).
var Scaffold = []sdkprovider.ScaffoldConfigLine{
	{Key: "azblobContainer", Value: "${env:RUNFABRIC_AZBLOB_CONTAINER}"},
	{Key: "azblobPrefix", Value: "runfabric/dev"},
}
