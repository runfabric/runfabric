export const handler = async () => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ service: "api", ok: true })
});
