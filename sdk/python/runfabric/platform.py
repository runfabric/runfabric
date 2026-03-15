import platform

def platform_key() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()

    if system == "darwin" and machine in ("arm64", "aarch64"):
        return "darwin-arm64"
    if system == "darwin" and machine in ("x86_64", "amd64"):
        return "darwin-amd64"
    if system == "linux" and machine in ("arm64", "aarch64"):
        return "linux-arm64"
    if system == "linux" and machine in ("x86_64", "amd64"):
        return "linux-amd64"
    if system == "windows" and machine in ("x86_64", "amd64"):
        return "windows-amd64"

    raise RuntimeError(f"unsupported platform {system}-{machine}")
