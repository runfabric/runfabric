"""HTTP response shaping."""
from typing import Any

def json_response(data: Any, status: int = 200) -> tuple[bytes, list[tuple[str, str]]]:
    import json
    return json.dumps(data).encode("utf-8"), [("Content-Type", "application/json")]
