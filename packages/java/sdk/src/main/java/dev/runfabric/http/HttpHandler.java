package dev.runfabric.http;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import dev.runfabric.Handler;
import dev.runfabric.HandlerContext;

import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

/**
 * Adapts a RunFabric Handler to a generic HTTP request/response (e.g. for use with HttpServlet or similar).
 */
public final class HttpHandler {
    private static final Gson GSON = new GsonBuilder().create();

    private final Handler handler;

    public HttpHandler(Handler handler) {
        this.handler = handler;
    }

    /**
     * Process an HTTP request: read JSON body as event, call handler, write JSON response.
     */
    public void handle(InputStreamReader bodyReader, OutputStream responseOut,
                      String stage, String functionName, String requestId) throws IOException {
        HandlerContext ctx = new HandlerContext(stage, functionName, requestId);
        Map<String, Object> event = new HashMap<>();
        try {
            @SuppressWarnings("unchecked")
            Map<String, Object> parsed = GSON.fromJson(bodyReader, Map.class);
            if (parsed != null) {
                event = parsed;
            }
        } catch (Exception ignored) {
            // use empty event
        }
        Map<String, Object> result = handler.handle(event, ctx);
        String json = GSON.toJson(result != null ? result : Map.of());
        responseOut.write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n".getBytes(StandardCharsets.UTF_8));
        responseOut.write(json.getBytes(StandardCharsets.UTF_8));
    }
}
