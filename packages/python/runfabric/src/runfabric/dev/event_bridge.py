"""Event bridge for dev server."""
from typing import Any

class DevEvent:
    def __init__(self, type: str, payload: Any = None):
        self.type = type
        self.payload = payload
