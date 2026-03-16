"""HTTP request shaping."""
from typing import Any

def parse_body(body: bytes) -> dict[str, Any]:
    import json
    return json.loads(body.decode("utf-8") or "{}")
