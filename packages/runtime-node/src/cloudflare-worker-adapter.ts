import type { UniversalHandler } from "@runfabric/core";

export function createCloudflareWorkerAdapter(handler: UniversalHandler) {
  return {
    async fetch(request: Request) {
      const url = new URL(request.url);
      const response = await handler({
        method: request.method,
        path: url.pathname,
        headers: Object.fromEntries(request.headers.entries()),
        query: Object.fromEntries(url.searchParams.entries()),
        body: await request.text(),
        raw: request
      });

      return new Response(response.body || "", {
        status: response.status,
        headers: response.headers || {}
      });
    }
  };
}
