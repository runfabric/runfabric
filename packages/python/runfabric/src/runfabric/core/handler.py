"""Handler type (event, context) -> response."""

from typing import Any, Awaitable, Callable

Context = dict[str, Any]
Handler = Callable[[dict, Context], Any | Awaitable[Any]]
