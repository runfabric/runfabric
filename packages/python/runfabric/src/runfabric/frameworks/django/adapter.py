"""Django adapter: wrap handler as view."""
from ...core import Handler

def runfabric_view(handler: Handler, method_post: bool = True):
    try:
        from django.http import JsonResponse
        from django.views.decorators.http import require_http_methods
        from django.views.decorators.csrf import csrf_exempt
    except ImportError:
        raise ImportError("pip install django")
    @csrf_exempt
    @require_http_methods(["POST"] if method_post else ["GET", "POST"])
    def view(request):
        import json
        body = request.body.decode("utf-8") or "{}"
        event = json.loads(body)
        context = {"stage": request.META.get("HTTP_X_STAGE", "dev"), "function_name": request.META.get("HTTP_X_FUNCTION", "handler")}
        result = handler(event, context)
        return JsonResponse(result)
    return view
