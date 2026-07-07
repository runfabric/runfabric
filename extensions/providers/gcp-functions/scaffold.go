package gcp

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a GCP Cloud Functions project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: gcp-functions — set GCP_PROJECT_ID; supports http, cron, queue, storage, pubsub",
}
