import type { UniversalHandler } from "@runfabric/sdk";

export const handler: UniversalHandler = async (req) => {
  let body: { title?: string } = {};
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

  const todo = {
    id: Date.now().toString(),
    title: body.title,
    done: false,
  };

  return {
    status: 201,
    headers: { "content-type": "application/json" },
    body: JSON.stringify(todo),
  };
};
