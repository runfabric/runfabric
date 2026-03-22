"""Handler type (event, context) -> response."""

from typing import Any, Awaitable, Callable, Union

Context = dict[str, Any]
Handler = Callable[[dict, Context], Union[Any, Awaitable[Any]]]
