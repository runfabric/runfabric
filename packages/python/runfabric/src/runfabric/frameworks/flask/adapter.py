"""Flask adapter: mount handler as route."""
from typing import Optional

from ...core import Handler

def mount(app, handler: Handler, path: str = "/", methods: Optional[list] = None):
    if methods is None:
        methods = ["POST"]
    try:
        from flask import request, jsonify
    except ImportError:
        raise ImportError("pip install flask")
    def route():
        event = request.get_json(force=True, silent=True) or {}
        context = {"stage": request.headers.get("X-Stage", "dev"), "function_name": request.headers.get("X-Function", "handler")}
        result = handler(event, context)
        return jsonify(result)
    app.add_url_rule(path, view_func=route, methods=methods)
