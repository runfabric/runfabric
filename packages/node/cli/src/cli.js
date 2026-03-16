#!/usr/bin/env node
"use strict";
/**
 * CLI entry: resolve platform binary and run it with argv.
 */
const { spawnSync } = require("child_process");
const { resolveBinary } = require("./platform");

const args = process.argv.slice(2);
let binary;
try {
    binary = resolveBinary();
} catch (e) {
    console.error(e.message);
    process.exit(1);
}
const result = spawnSync(binary, args, { stdio: "inherit" });
process.exit(result.status != null ? result.status : 1);
