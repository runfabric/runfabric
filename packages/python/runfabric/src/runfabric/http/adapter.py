"""Adapt (event, context) -> response to ASGI/WSGI."""
import json
from ..core import Handler

def create_asgi_handler(handler: Handler):
    async def asgi(scope, receive, send):
        if scope["type"] != "http":
            await send({"type": "http.response.start", "status": 400})
            await send({"type": "http.response.body", "body": b"{}"})
            return
        body = b""
        while True:
            m = await receive()
            body += m.get("body", b"") or b""
            if not m.get("more_body", False):
                break
        event = json.loads(body.decode("utf-8") or "{}")
        context = {"stage": "dev", "function_name": "handler"}
        result = handler(event, context)
        if hasattr(result, "__await__"):
            result = await result
        out = json.dumps(result).encode("utf-8")
        await send({"type": "http.response.start", "status": 200, "headers": [[b"content-type", b"application/json"]]})
        await send({"type": "http.response.body", "body": out})
    return asgi

def create_wsgi_handler(handler: Handler):
    def wsgi(environ, start_response):
        from .request import parse_body
        try:
            length = int(environ.get("CONTENT_LENGTH", 0) or 0)
            body = environ["wsgi.input"].read(length).decode("utf-8") if length else "{}"
        except (ValueError, KeyError):
            body = "{}"
        event = json.loads(body)
        context = {"stage": "dev", "function_name": "handler"}
        result = handler(event, context)
        start_response("200 OK", [("Content-Type", "application/json")])
        return [json.dumps(result).encode("utf-8")]
    return wsgi
