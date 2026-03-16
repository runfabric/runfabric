"""CLI entry: invoke engine binary."""
import sys
import subprocess
import os

def _platform_key():
    import platform
    m = platform.machine().lower()
    a = "arm64" if "arm" in m or m == "aarch64" else "amd64"
    s = sys.platform
    if s == "darwin":
        return f"darwin-{a}"
    if s == "linux":
        return f"linux-{a}"
    if s == "win32":
        return f"windows-{a}"
    raise RuntimeError(f"Unsupported platform {s}")

def _resolve_binary():
    key = _platform_key()
    suffix = ".exe" if os.name == "nt" else ""
    name = f"runfabric-{key}{suffix}"
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", ".."))
    local = os.path.join(root, "bin", name)
    home = os.path.join(os.path.expanduser("~"), ".runfabric", "bin", name)
    if os.path.exists(local):
        return local
    if os.path.exists(home):
        return home
    raise RuntimeError(f"runfabric binary not found for {key}. Build from engine/ or place {name} in bin/ or ~/.runfabric/bin/")

def main():
    try:
        bin_path = _resolve_binary()
        sys.exit(subprocess.call([bin_path] + sys.argv[1:]))
    except Exception as e:
        print(f"runfabric: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
