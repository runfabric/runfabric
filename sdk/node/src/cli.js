#!/usr/bin/env node
const { spawnSync } = require("node:child_process");
const { resolveBinary } = require("./bin");

try {
    const bin = resolveBinary();
    const args = process.argv.slice(2);
    const res = spawnSync(bin, args, { stdio: "inherit" });
    process.exit(res.status ?? 1);
} catch (error) {
    console.error(`runfabric start error: ${error.message}`);
    process.exit(1);
}
