"""Structural tests for RunFabric Python CLI and programmatic API."""
import pytest


def test_platform_key_format():
    """_platform_key returns darwin-arch, linux-arch, or windows-arch."""
    from runfabric.cli import _platform_key
    key = _platform_key()
    assert key is not None
    assert "-" in key
    parts = key.split("-")
    assert len(parts) == 2
    assert parts[0] in ("darwin", "linux", "windows")
    assert parts[1] in ("amd64", "arm64")


def test_programmatic_api_exports():
    """run, plan, deploy, build are exposed and callable."""
    import runfabric
    assert hasattr(runfabric, "run")
    assert hasattr(runfabric, "plan")
    assert hasattr(runfabric, "deploy")
    assert hasattr(runfabric, "build")
    assert callable(runfabric.run)
    assert callable(runfabric.plan)
    assert callable(runfabric.deploy)
    assert callable(runfabric.build)
