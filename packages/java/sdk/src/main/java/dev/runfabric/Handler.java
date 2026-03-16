package dev.runfabric;

import java.util.Map;

/**
 * RunFabric handler contract: (event, context) -> response map.
 */
@FunctionalInterface
public interface Handler {
    /**
     * Handle an invocation event.
     *
     * @param event   event payload (e.g. JSON body as map)
     * @param context request context (stage, function name, request ID)
     * @return response map (serialized as JSON by the runtime)
     */
    Map<String, Object> handle(Map<String, Object> event, HandlerContext context);
}
