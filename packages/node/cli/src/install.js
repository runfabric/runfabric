const path = require('path');
const fs = require('fs');
const { platformKey, binarySuffix } = require('./platform');

function install() {
    let key;
    try {
        key = platformKey();
    } catch (e) {
        console.warn('RunFabric CLI: Unsupported platform. Use "make build-platform" from repo or install binary manually.');
        return;
    }

    const suffix = binarySuffix();
    const binName = `runfabric-${key}${suffix}`;
    const pkgBin = path.join(__dirname, '..', 'bin', binName);
    const repoBin = path.join(__dirname, '..', '..', '..', '..', 'bin', binName);

    if (fs.existsSync(pkgBin)) {
        try {
            fs.chmodSync(pkgBin, 0o755);
        } catch (_) {}
        return;
    }
    if (fs.existsSync(repoBin)) return;

    console.log(`RunFabric CLI: platform ${key}, binary name ${binName}`);
    console.log('RunFabric CLI: Run "make build-platform" in repo to build, or install a release that includes bundled binaries.');
}

install();
