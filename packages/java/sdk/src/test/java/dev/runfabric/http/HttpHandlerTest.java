package dev.runfabric.http;

import dev.runfabric.Handler;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

class HttpHandlerTest {

    @Test
    void handleWritesJsonResponse() throws Exception {
        Handler h = (event, context) -> java.util.Map.of("ok", true, "stage", context.getStage());
        HttpHandler adapter = new HttpHandler(h);
        String body = "{\"x\":1}";
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        adapter.handle(
            new InputStreamReader(new ByteArrayInputStream(body.getBytes(StandardCharsets.UTF_8))),
            out,
            "dev", "api", "req-1"
        );
        String response = out.toString(StandardCharsets.UTF_8);
        assertTrue(response.contains("200 OK"));
        assertTrue(response.contains("application/json"));
        assertTrue(response.contains("\"ok\":true"));
        assertTrue(response.contains("\"stage\":\"dev\""));
    }
}
