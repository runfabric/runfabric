"use strict";
// Smoke test: SDK loads and createHandler works.
const { createHandler, createHttpHandler, loadHandler } = require("../src");
const h = createHandler((event) => ({ body: JSON.stringify({ ok: true }) }));
if (typeof h !== "function" || typeof h.mountExpress !== "function") {
  console.error("createHandler failed");
  process.exit(1);
}
if (typeof createHttpHandler !== "function" || typeof loadHandler !== "function") {
  console.error("createHttpHandler or loadHandler missing");
  process.exit(1);
}
console.log("Smoke test passed: @runfabric/sdk OK");
process.exit(0);
