package cloudflare

import sdkprovider "github.com/runfabric/runfabric/plugin-sdk/go/provider"

// Scaffold declares how `runfabric init` scaffolds a Cloudflare Workers project.
// Workers deploy a single worker.js script (not a Lambda-style handler), so this
// overrides the entry file/ref and handler body for JS scaffolds.
var Scaffold = sdkprovider.Scaffold{
	Comment:   "Provider: cloudflare-workers — set CLOUDFLARE_*; supports http, cron",
	Entry:     "worker.fetch",
	EntryFile: "worker.js",
	Sample:    workerSample,
}

// workerSample is the Cloudflare Workers module scaffold (fetch handler).
const workerSample = `export default {
  async fetch(request) {
    const body = await request.text();
    return new Response(
      JSON.stringify({ message: "Hello from RunFabric", echo: body }),
      { headers: { "content-type": "application/json" } },
    );
  },
};
`
