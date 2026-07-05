import type { UniversalHandler } from "@runfabric/sdk";

/**
 * Workflow code-step target: invoked by the `archive` step of the `ai-review`
 * workflow (not by HTTP). The step's payload arrives as the request body; the
 * returned JSON becomes the step's recorded output in the run journal.
 */
export const handler: UniversalHandler = async (req) => {
  let payload: Record<string, unknown> = {};
  try {
    payload = JSON.parse(req.body ?? "{}");
  } catch {
    payload = {};
  }

  return {
    status: 200,
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      archived: true,
      archivedAt: new Date().toISOString(),
      received: payload,
    }),
  };
};
