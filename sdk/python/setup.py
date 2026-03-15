import os
import sys
import subprocess
from setuptools import setup, find_packages
from setuptools.command.install import install

class PostInstallCommand(install):
    def run(self):
        install.run(self)
        # Add downloading code here logic mapped to runfabric repository
        print("RunFabric PIP Wrapper: Starting post setup hook...")
        platform = sys.platform
        print(f"RunFabric PIP Wrapper: Attempting to resolve binaries for {platform}...")
        
        # Determine the user site folder path or target bin installation path for system level commands.
        # This proxy download handler will securely fetch OS-mapped versions of `runfabric` when published.
        print("RunFabric PIP Wrapper: Local execution scaffolding setup.")

setup(
    name="runfabric",
    version="1.0.0",
    description="RunFabric Framework CLI & SDK Python Wrapper",
    packages=find_packages(),
    cmdclass={
        'install': PostInstallCommand,
    },
    entry_points={
        "console_scripts": [
            "runfabric=runfabric.cli:main",
        ],
    },
)
