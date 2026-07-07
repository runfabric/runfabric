package ibm

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds an IBM OpenWhisk project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: ibm-openwhisk — see docs for credentials and trigger support",
}
