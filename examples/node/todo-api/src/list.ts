import type { UniversalHandler } from "@runfabric/sdk";

const todos = [
  { id: "1", title: "Buy groceries", done: false },
  { id: "2", title: "Read docs", done: true },
];

export const handler: UniversalHandler = async (req) => {
  const done = req.query?.done;
  const items = done !== undefined
    ? todos.filter((t) => String(t.done) === done)
    : todos;

  return {
    status: 200,
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ items, total: items.length }),
  };
};
