package dev.runfabric;

import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class HandlerTest {

    @Test
    void handlerReturnsResponse() {
        Handler h = (event, context) -> Map.of(
            "message", "hello",
            "stage", context.getStage()
        );
        HandlerContext ctx = new HandlerContext("dev", "api", "req-1");
        Map<String, Object> out = h.handle(Map.of("name", "world"), ctx);
        assertEquals("hello", out.get("message"));
        assertEquals("dev", out.get("stage"));
    }

    @Test
    void contextDefaultsStageToDev() {
        HandlerContext ctx = new HandlerContext(null, null, null);
        assertEquals("dev", ctx.getStage());
    }
}
