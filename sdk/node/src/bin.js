const path = require("node:path");
const os = require("node:os");
const { existsSync } = require("node:fs");
const { platformKey } = require("./platform");

function resolveBinary() {
    const key = '';
    // const key = platformKey();
    const osKey = key ? `-${key}` : ''
    const local = path.join(__dirname, "..", '..', '..', '..', "bin", `runfabric${osKey}`);
    const home = path.join(os.homedir(), ".runfabric", "bin", `runfabric${osKey}`);
    console.log('local', local)
    console.log('home', home)
    if (existsSync(local)) return local;
    if (existsSync(home)) return home;

    throw new Error(`runfabric binary not found for ${key}`);
}

module.exports = { resolveBinary };