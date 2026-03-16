"use strict";
const os = require("os");
const path = require("path");
const fs = require("fs");

function platformKey() {
    const p = process.platform;
    const a = process.arch;
    if (p === "darwin" && (a === "arm64" || a === "x64")) return a === "arm64" ? "darwin-arm64" : "darwin-amd64";
    if (p === "linux" && (a === "arm64" || a === "x64")) return a === "arm64" ? "linux-arm64" : "linux-amd64";
    if (p === "win32" && (a === "arm64" || a === "x64")) return a === "arm64" ? "windows-arm64" : "windows-amd64";
    throw new Error("Unsupported platform " + p + "-" + a);
}

function binarySuffix() {
    return process.platform === "win32" ? ".exe" : "";
}

function resolveBinary() {
    const key = platformKey();
    const suffix = binarySuffix();
    const binName = "runfabric-" + key + suffix;
    const dirs = [
        path.join(__dirname, "..", "bin"),
        path.join(__dirname, "..", "..", "..", "..", "bin"),
        path.join(os.homedir(), ".runfabric", "bin"),
    ];
    for (const dir of dirs) {
        const full = path.join(dir, binName);
        if (fs.existsSync(full)) return full;
    }
    throw new Error("runfabric binary not found for " + key + ". Run make build-platform or install a release with bundled binaries.");
}

module.exports = { platformKey, binarySuffix, resolveBinary };
