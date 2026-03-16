/**
 * Function invocation helpers (event, context) => response and loadHandler.
 */
const raw = require("../adapters/raw");
module.exports = { createHttpHandler: raw.createHttpHandler, loadHandler: raw.loadHandler };
