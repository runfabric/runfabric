package netlify

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a Netlify project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: netlify — set provider token; supports http, cron",
}
