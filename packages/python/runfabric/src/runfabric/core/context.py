"""Request context (stage, function name, request ID)."""

from typing import Any, Optional

def make_context(request_id: Optional[str] = None, stage: str = "dev", function_name: str = "handler") -> dict[str, Any]:
    return {"request_id": request_id, "stage": stage, "function_name": function_name}
