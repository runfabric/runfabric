const path = require("node:path");
const os = require("node:os");
const { existsSync } = require("node:fs");
const { platformKey, binarySuffix } = require("./platform");

function resolveBinary() {
    const key = platformKey();
    const suffix = binarySuffix();
    const baseName = `runfabric-${key}${suffix}`;
    const pkgBin = path.join(__dirname, "..", "bin", baseName);
    const repoBin = path.join(__dirname, "..", "..", "..", "..", "bin", baseName);
    const homeBin = path.join(os.homedir(), ".runfabric", "bin", baseName);
    if (existsSync(pkgBin)) return pkgBin;
    if (existsSync(repoBin)) return repoBin;
    if (existsSync(homeBin)) return homeBin;

    throw new Error(`runfabric binary not found for ${key}. Install a release with bundled binaries or run: make build-platform (or place ${baseName} in bin/ or ~/.runfabric/bin/)`);
}

module.exports = { resolveBinary };
