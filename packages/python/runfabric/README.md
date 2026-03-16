# RunFabric Python SDK

CLI wrapper and programmatic API for the RunFabric engine. Requires the engine binary (build from `engine/` or place in `bin/` or `~/.runfabric/bin/`).

## CLI

```bash
runfabric doctor --config runfabric.yml
runfabric deploy --stage dev
```

## Programmatic API (aligned with Go CLI)

```python
import runfabric

# Run any command; returns parsed JSON when the command supports --json
runfabric.run("plan", config="runfabric.yml", stage="dev")
runfabric.run("doctor", config="runfabric.yml", stage="dev")

# Convenience methods
runfabric.plan(config="runfabric.yml", stage="dev")
runfabric.deploy(config="runfabric.yml", stage="dev", rollback_on_failure=True)
runfabric.build(config="runfabric.yml", stage="dev", out_dir="dist")
```

Binary is resolved from repo `bin/` or `~/.runfabric/bin/` (see `runfabric.cli._resolve_binary()`).

## Tests

```bash
pip install -e ".[dev]"
pytest tests/ -v
```

## Optional: FastAPI / Flask / Django

Install optional dependencies for framework adapters:

```bash
pip install runfabric[fastapi]
pip install runfabric[flask]
pip install runfabric[django]
```
