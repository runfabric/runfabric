package gcs

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares the backend: config block this state backend contributes to
// `runfabric init` (co-located with the backend, like a provider's Scaffold).
var Scaffold = []sdkprovider.ScaffoldConfigLine{
	{Key: "gcsBucket", Value: "${env:RUNFABRIC_GCS_BUCKET}"},
	{Key: "gcsPrefix", Value: "runfabric/dev"},
}
