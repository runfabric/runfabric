import type { UniversalHandler } from "@runfabric/sdk";

/**
 * HTTP entry point: accept a document for review. In a real app this would
 * enqueue the payload and kick off the `ai-review` workflow; here it just
 * echoes back an accepted receipt so the example is self-contained.
 */
export const handler: UniversalHandler = async (req) => {
  let body: { title?: string; content?: string } = {};
  try {
    body = JSON.parse(req.body ?? "{}");
  } catch {
    return {
      status: 400,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ error: "invalid json" }),
    };
  }

  if (!body.title) {
    return {
      status: 422,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ error: "title is required" }),
    };
  }

  return {
    status: 202,
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      accepted: true,
      documentId: Date.now().toString(),
      title: body.title,
    }),
  };
};
