/**
 * HTTP helpers for RunFabric (e.g. request/response shaping for call-local).
 * Re-exports raw createHttpHandler for HTTP usage.
 */
const { createHttpHandler } = require("../adapters/raw");
module.exports = { createHttpHandler };
