package fly

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a Fly Machines project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: fly-machines — set FLY_*; http only",
}
