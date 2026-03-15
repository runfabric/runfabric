module.exports = function platformKey() {
    const p = process.platform;
    const a = process.arch;

    if (p === "darwin" && a === "arm64") return "darwin-arm64";
    if (p === "darwin" && a === "x64") return "darwin-amd64";
    if (p === "linux" && a === "arm64") return "linux-arm64";
    if (p === "linux" && a === "x64") return "linux-amd64";
    if (p === "win32" && a === "x64") return "windows-amd64";

    throw new Error(`unsupported platform ${p}-${a}`);
};
