import type { UniversalHandler } from "@runfabric/sdk";

const todos: Record<string, { id: string; title: string; done: boolean }> = {
  "1": { id: "1", title: "Buy groceries", done: false },
  "2": { id: "2", title: "Read docs", done: true },
};

export const handler: UniversalHandler = async (req) => {
  const id = req.pathParams?.id ?? req.path.split("/").pop() ?? "";
  const todo = todos[id];

  if (!todo) {
    return {
      status: 404,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ error: "not found", id }),
    };
  }

  return {
    status: 200,
    headers: { "content-type": "application/json" },
    body: JSON.stringify(todo),
  };
};
