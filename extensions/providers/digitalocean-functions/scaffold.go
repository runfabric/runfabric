package digitalocean

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a DigitalOcean Functions project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: digitalocean-functions — see docs for credentials and trigger support",
}
