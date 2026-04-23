package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.*;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;

import org.junit.jupiter.api.Test;

/**
 * Tests for FunctionHandler interface and implementations.
 */
class FunctionHandlerTest {

    @Test
    void syncFunctionHandlerExecutes() throws Exception {
        FunctionHandler handler = (context, payload) -> {
            return "{\"result\":\"success\"}";
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";
        String result = handler.handle(context, "{\"input\":\"test\"}");

        assertEquals("{\"result\":\"success\"}", result);
    }

    @Test
    void functionHandlerThrowsException() {
        FunctionHandler handler = (context, payload) -> {
            throw new RuntimeException("Test exception");
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";

        assertThrows(RuntimeException.class, () -> {
            handler.handle(context, "{}");
        });
    }

    @Test
    void functionHandlerWithEmptyPayload() throws Exception {
        FunctionHandler handler = (context, payload) -> {
            return "";
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";
        String result = handler.handle(context, "");

        assertEquals("", result);
    }

    @Test
    void functionHandlerWithNullPayload() throws Exception {
        FunctionHandler handler = (context, payload) -> {
            return "{\"result\":\"ok\"}";
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";
        String result = handler.handle(context, null);

        assertEquals("{\"result\":\"ok\"}", result);
    }

    @Test
    void functionHandlerWithLargePayload() throws Exception {
        FunctionHandler handler = (context, payload) -> {
            return payload; // Echo back the payload
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < 10000; i++) {
            sb.append("x");
        }
        String largePayload = sb.toString();

        String result = handler.handle(context, largePayload);

        assertEquals(largePayload, result);
    }

    @Test
    void functionHandlerContextParse() throws Exception {
        FunctionHandler handler = (context, payload) -> {
            // Parse context to verify it contains correct info
            assertTrue(context.contains("\"functionId\":\"test-function\""));
            assertTrue(context.contains("\"gameId\":\"test-game\""));
            assertTrue(context.contains("\"serviceId\":\"test-service\""));
            return "{\"ok\":true}";
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";
        handler.handle(context, "{}");
    }

    @Test
    void functionHandlerChaining() throws Exception {
        FunctionHandler handler1 = (context, payload) -> {
            return "{\"step\":\"1\"}";
        };

        FunctionHandler handler2 = (context, payload) -> {
            String previous = payload;
            return previous.replace("}", ",\"step\":\"2\"}");
        };

        String context = "{\"functionId\":\"test-function\",\"gameId\":\"test-game\",\"serviceId\":\"test-service\"}";

        String result1 = handler1.handle(context, "{}");
        String result2 = handler2.handle(context, result1);

        assertEquals("{\"step\":\"1\",\"step\":\"2\"}", result2);
    }
}
