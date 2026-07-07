package vercel

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a Vercel project.
var Scaffold = sdkprovider.Scaffold{
	Comment: "Provider: vercel — set provider token; supports http, cron",
}
