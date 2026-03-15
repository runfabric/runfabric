import sys
import subprocess
import os
from .platform import platform_key

def resolve_binary():
    key = platform_key()
    os_key = f"-{key}" if key else ""
    local = os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", "bin", f"runfabric{os_key}")
    local = os.path.abspath(local)
    if os.name == 'nt':
        local += '.exe'
    
    home = os.path.join(os.path.expanduser("~"), ".runfabric", "bin", f"runfabric{os_key}")
    if os.name == 'nt':
        home += '.exe'
        
    if os.path.exists(local):
        return local
    if os.path.exists(home):
        return home
        
    raise RuntimeError(f"runfabric binary not found for {key}")

def main():
    try:
        bin_path = resolve_binary()
        sys.exit(subprocess.call([bin_path] + sys.argv[1:]))
    except Exception as e:
        print(f"runfabric start error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
