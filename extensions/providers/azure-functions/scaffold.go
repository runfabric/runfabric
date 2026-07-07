package azure

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds an Azure Functions project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: azure-functions — set AZURE_*; supports http, cron, queue, storage",
}
