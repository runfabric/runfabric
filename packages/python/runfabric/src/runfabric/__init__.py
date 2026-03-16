"""RunFabric Python SDK."""
from .core import Handler, Context
from .http import create_asgi_handler, create_wsgi_handler
from .frameworks import fastapi_mount, flask_mount, runfabric_view

# Programmatic API: invoke engine binary (aligned with Go CLI)
def run(command, *args, config="runfabric.yml", stage="dev", json_output=True):
    """Run runfabric <command> with optional --config, --stage; returns parsed JSON if json_output."""
    import subprocess
    import json
    from .cli import _resolve_binary
    bin_path = _resolve_binary()
    cmd = [bin_path, command, "--config", config, "--stage", stage]
    cmd.extend(str(a) for a in args)
    if json_output:
        cmd.append("--json")
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
    if result.returncode != 0:
        raise RuntimeError(result.stderr or result.stdout or f"runfabric {command} failed")
    if json_output and result.stdout:
        return json.loads(result.stdout)
    return result.stdout or {}

def plan(config="runfabric.yml", stage="dev"):
    """Run runfabric plan; returns parsed output."""
    return run("plan", config=config, stage=stage)

def deploy(config="runfabric.yml", stage="dev", rollback_on_failure=None):
    """Run runfabric deploy; returns parsed output."""
    args = []
    if rollback_on_failure is False:
        args.append("--no-rollback-on-failure")
    elif rollback_on_failure is True:
        args.append("--rollback-on-failure")
    return run("deploy", *args, config=config, stage=stage)

def build(config="runfabric.yml", stage="dev", out_dir=""):
    """Run runfabric build; returns parsed output."""
    args = ["--out", out_dir] if out_dir else []
    return run("build", *args, config=config, stage=stage)

__all__ = [
    "Handler", "Context",
    "create_asgi_handler", "create_wsgi_handler",
    "fastapi_mount", "flask_mount", "runfabric_view",
    "run", "plan", "deploy", "build",
]
