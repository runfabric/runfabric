package kubernetes

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a Kubernetes project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: kubernetes — set KUBECONFIG; supports http, cron",
}
