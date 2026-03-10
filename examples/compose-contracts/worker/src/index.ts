export const handler = async (event: unknown) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ service: "worker", received: event ?? null })
});
