import subprocess
import json
from .cli import resolve_binary

def run_command(command, *args):
    """Executes a internal core binary command targeting runfabric architecture"""
    bin_path = resolve_binary()
    full_args = [bin_path, command] + list(args) + ["--json"]
    
    result = subprocess.run(full_args, capture_output=True, text=True)
    
    if result.returncode != 0:
        raise RuntimeError(f"RunFabric execution failed: {result.stderr}")
        
    try:
        output = json.loads(result.stdout)
        if not output.get("ok"):
            raise RuntimeError(output.get("error", "Unknown error"))
        return output.get("result")
    except json.JSONDecodeError:
        raise RuntimeError(f"RunFabric returned malformed JSON: {result.stdout}")

def deploy(stage="dev", config_path="runfabric.yml", no_rollback=False):
    args = ["--stage", stage, "--config", config_path]
    if no_rollback:
        args.append("--no-rollback-on-failure")
    return run_command("deploy", *args)

def build(stage="dev", config_path="runfabric.yml", out_dir=None):
    args = ["--stage", stage, "--config", config_path]
    if out_dir:
        args.extend(["--out", out_dir])
    return run_command("build", *args)
