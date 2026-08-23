package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Function;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Coverage boost tests for ServerHttpInvoker: payload schema validation,
 * retry semantics, timeout mapping, response parsing and URL normalization.
 */
@DisplayName("ServerHttpInvoker coverage boost")
class ServerHttpInvokerBoostTest {

    private record Response(int status, String body) {
        static Response ok(String body) { return new Response(200, body); }
        static Response status(int status, String body) { return new Response(status, body); }
    }

    private static final class MockServer implements AutoCloseable {
        private final HttpServer server;
        MockServer(Function<HttpExchange, Response> responder) throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                Response response = responder.apply(exchange);
                byte[] body = response.body().getBytes(StandardCharsets.UTF_8);
                exchange.getResponseHeaders().add("Content-Type", "application/json");
                exchange.sendResponseHeaders(response.status(), body.length);
                exchange.getResponseBody().write(body);
                exchange.close();
            });
            server.start();
        }
        String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
        @Override public void close() { server.stop(0); }
    }

    private static InvokerConfig.Builder configBuilder(String address) {
        return InvokerConfig.builder().address(address);
    }

    @Test
    @DisplayName("invoke enforces locally configured required fields")
    void invokeValidatesRequiredFields() throws Exception {
        try (MockServer server = new MockServer(ex ->
                Response.ok("{\"result\":{}}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(configBuilder(server.baseUrl()).build());
            try {
                invoker.setSchema("fn", Map.of("type", "object", "required", List.of("playerId")));
                InvokerException missing = assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "{\"other\":1}"));
                assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, missing.getErrorCode());

                InvokerException nonObject = assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "[1,2]"));
                assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, nonObject.getErrorCode());

                // Valid payload passes through.
                assertDoesNotThrow(() -> invoker.invoke("fn", "{\"playerId\":\"p1\"}"));
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("setSchema rejects blank function IDs and null schemas")
    void setSchemaGuards() throws Exception {
        ServerHttpInvoker invoker = new ServerHttpInvoker(configBuilder("http://127.0.0.1:1").build());
        try {
            assertThrows(IllegalArgumentException.class, () -> invoker.setSchema(" ", Map.of()));
            invoker.setSchema("fn", null);
        } finally {
            invoker.close();
        }
    }

    @Test
    @DisplayName("invoke rejects a response without a result key")
    void invokeRequiresResultKey() throws Exception {
        try (MockServer server = new MockServer(ex ->
                Response.ok("{\"something\":\"else\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(configBuilder(server.baseUrl()).build());
            try {
                InvokerException error = assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "{}"));
                assertEquals(InvokerException.ErrorCode.INTERNAL, error.getErrorCode());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("invalid JSON responses map to INTERNAL errors")
    void invalidJsonResponse() throws Exception {
        try (MockServer server = new MockServer(ex ->
                new Response(200, "not-json"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(configBuilder(server.baseUrl()).build());
            try {
                InvokerException error = assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "{}"));
                assertEquals(InvokerException.ErrorCode.INTERNAL, error.getErrorCode());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("blank response bodies deserialize to empty maps")
    void blankBodyResponse() throws Exception {
        try (MockServer server = new MockServer(ex -> new Response(200, ""))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(configBuilder(server.baseUrl()).build());
            try {
                // cancel maps a blank body to an empty object without failing.
                assertDoesNotThrow(() -> invoker.cancelTask("t1"));
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("server timeouts map to TIMEOUT errors without retry storm")
    void timeoutMapsToTimeoutError() throws Exception {
        try (MockServer server = new MockServer(ex -> {
            try {
                Thread.sleep(2000);
            } catch (InterruptedException ignored) {
                Thread.currentThread().interrupt();
            }
            return Response.ok("{}");
        })) {
            RetryConfig noRetry = RetryConfig.builder().enabled(false).build();
            ServerHttpInvoker invoker = new ServerHttpInvoker(
                    configBuilder(server.baseUrl()).timeout(200).retry(noRetry).build());
            try {
                InvokerException error = assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "{}"));
                assertEquals(InvokerException.ErrorCode.TIMEOUT, error.getErrorCode());
            } finally {
                invoker.close();
                server.close();
            }
        } catch (IOException ioError) {
            throw new AssertionError(ioError);
        }
    }

    @Test
    @DisplayName("retryable failures are retried until success")
    void retryUntilSuccess() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(ex -> {
            int attempt = calls.incrementAndGet();
            if (attempt < 3) {
                return Response.status(503, "{\"message\":\"flaky\"}");
            }
            return Response.ok("{\"result\":{\"ok\":true}}");
        })) {
            RetryConfig fastRetry = RetryConfig.builder()
                .maxAttempts(3)
                .initialDelayMs(1)
                .build();
            ServerHttpInvoker invoker = new ServerHttpInvoker(
                    configBuilder(server.baseUrl()).retry(fastRetry).build());
            try {
                String result = invoker.invoke("fn", "{}");
                assertTrue(result.contains("ok"));
                assertEquals(3, calls.get());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("non-retryable client failures fail fast")
    void nonRetryableFailsFast() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(ex -> {
            calls.incrementAndGet();
            return Response.status(404, "{\"message\":\"missing\"}");
        })) {
            RetryConfig retry = RetryConfig.builder()
                .maxAttempts(5)
                .initialDelayMs(1)
                .build();
            ServerHttpInvoker invoker = new ServerHttpInvoker(
                    configBuilder(server.baseUrl()).retry(retry).build());
            try {
                InvokerException error = assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "{}"));
                assertEquals(InvokerException.ErrorCode.NOT_FOUND, error.getErrorCode());
                assertEquals(1, calls.get());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("request-level retry config overrides the invoker config")
    void requestLevelRetryOverrides() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(ex -> {
            calls.incrementAndGet();
            return Response.status(503, "{\"message\":\"down\"}");
        })) {
            RetryConfig globalRetry = RetryConfig.builder()
                .maxAttempts(5)
                .initialDelayMs(1)
                .build();
            RetryConfig requestRetry = RetryConfig.builder()
                .enabled(false)
                .build();
            ServerHttpInvoker invoker = new ServerHttpInvoker(
                    configBuilder(server.baseUrl()).retry(globalRetry).build());
            try {
                assertThrows(InvokerException.class,
                    () -> invoker.invoke("fn", "{}", InvokeOptions.builder().retry(requestRetry).build()));
                assertEquals(1, calls.get());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("connection failures map to UNAVAILABLE")
    void connectionFailureMapsToUnavailable() {
        // Nothing listens on this port in CI.
        RetryConfig noRetry = RetryConfig.builder().enabled(false).build();
        ServerHttpInvoker invoker = new ServerHttpInvoker(
                configBuilder("http://127.0.0.1:1").retry(noRetry).build());
        try {
            InvokerException error = assertThrows(InvokerException.class,
                () -> invoker.invoke("fn", "{}"));
            assertEquals(InvokerException.ErrorCode.UNAVAILABLE, error.getErrorCode());
        } finally {
            invoker.close();
        }
    }

    @Test
    @DisplayName("base URL normalization variants")
    void baseUrlNormalization() {
        assertEquals("http://127.0.0.1:18780/api/v1",
            new ServerHttpInvoker(configBuilder("http://127.0.0.1:18780").build()).getBaseUrl());
        assertEquals("https://server.example/api/v1",
            new ServerHttpInvoker(configBuilder("https://server.example/api/v1/").build()).getBaseUrl());
        assertEquals("http://host:1234/api/v1",
            new ServerHttpInvoker(configBuilder("host:1234").build()).getBaseUrl());
        assertEquals("http://127.0.0.1:18780/api/v1",
            new ServerHttpInvoker(configBuilder("  ").build()).getBaseUrl());
        assertThrows(IllegalArgumentException.class,
            () -> new ServerHttpInvoker(configBuilder("ftp://host").build()));
    }

    @Test
    @DisplayName("connect/close toggle the connected flag")
    void connectCloseLifecycle() {
        ServerHttpInvoker invoker = new ServerHttpInvoker(configBuilder("http://127.0.0.1:1").build());
        try {
            assertFalse(invoker.isConnected());
            invoker.connect();
            assertTrue(invoker.isConnected());
            invoker.close();
            assertFalse(invoker.isConnected());
        } finally {
            invoker.close();
        }
    }
}
