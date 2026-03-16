"""FastAPI adapter: mount handler as route."""
from ...core import Handler

def mount(app, handler: Handler, path: str = "/", method: str = "post"):
    try:
        from fastapi import Request
        from fastapi.responses import JSONResponse
    except ImportError:
        raise ImportError("pip install fastapi")
    async def route(request: Request):
        import json
        body = await request.body()
        event = json.loads(body.decode("utf-8") or "{}")
        context = {"stage": request.headers.get("x-stage", "dev"), "function_name": request.headers.get("x-function", "handler")}
        result = handler(event, context)
        if hasattr(result, "__await__"):
            result = await result
        return JSONResponse(content=result)
    m = method.upper()
    if m == "POST":
        app.post(path)(route)
    else:
        app.add_api_route(path, route, methods=[m])
