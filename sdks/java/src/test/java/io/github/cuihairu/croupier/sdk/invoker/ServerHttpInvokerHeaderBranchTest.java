package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Second-wave branch fillers for ServerHttpInvoker. */
class ServerHttpInvokerHeaderBranchTest {

    private record Response(int status, String body) {
        static Response ok(String body) { return new Response(200, body); }
        static Response status(int status, String body) { return new Response(status, body); }
    }

    @FunctionalInterface
    private interface Responder { Response respond(HttpExchange exchange) throws IOException; }

    private static final class MockServer implements AutoCloseable {
        private final HttpServer server;
        MockServer(Responder responder) throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                Response response = responder.respond(exchange);
                byte[] body = response.body().getBytes(StandardCharsets.UTF_8);
                exchange.sendResponseHeaders(response.status(), body.length);
                exchange.getResponseBody().write(body);
                exchange.close();
            });
            server.start();
        }
        String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
        String hostOnlyUrl() { return "127.0.0.1:" + server.getAddress().getPort(); }
        @Override public void close() { server.stop(0); }
    }

    private static ServerHttpInvoker invoker(String address, RetryConfig retry) {
        return new ServerHttpInvoker(InvokerConfig.builder()
            .address(address).taskPollIntervalMs(1)
            .retry(retry == null ? RetryConfig.builder().enabled(false).build() : retry)
            .build());
    }

    @Test
    void existingIdempotencyHeaderSuppressesOptionKey() throws Exception {
        List<String> keys = new ArrayList<>();
        try (MockServer server = new MockServer(exchange -> {
            keys.add(exchange.getRequestHeaders().getFirst("Idempotency-Key"));
            return Response.ok("{\"result\":\"ok\"}");
        })) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            invoker.invoke("fn", "{}", InvokeOptions.builder()
                .idempotencyKey("from-option")
                .header("Idempotency-Key", "from-header")
                .build());
            assertEquals("from-header", keys.get(0));
        }
    }

    @Test
    void schemeLessAddressDefaultsToHttp() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server.hostOnlyUrl(), null);
            assertEquals("http://" + server.hostOnlyUrl() + "/api/v1", invoker.getBaseUrl());
        }
    }

    @Test
    void zeroMaxDelayRetryDelayIsClamped() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.status(503, "{\"message\":\"busy\"}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), RetryConfig.builder()
                .enabled(true).maxAttempts(3).initialDelayMs(1).maxDelayMs(0)
                .backoffMultiplier(0).jitterFactor(0).build());
            long start = System.nanoTime();
            assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            long elapsedMs = (System.nanoTime() - start) / 1_000_000;
            assertTrue(elapsedMs < 500, "zero maxDelayMs must clamp delays, took " + elapsedMs + "ms");
        }
    }

    @Test
    void taskStatusBlankFieldsFallBackToDefaults() throws Exception {
        try (MockServer server = new MockServer(exchange ->
            Response.ok("{\"id\":\"  \",\"status\":\"\", \"result\":{\"done\":true}}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            TaskStatusInfo status = invoker.getTaskStatus("task-x");
            assertEquals("task-x", status.taskId());
            assertEquals("unknown", status.status());
            assertEquals("{\"done\":true}", status.result());
        }
    }

    @Test
    void nonStringErrorMessageFallsBackToBody() throws Exception {
        try (MockServer server = new MockServer(exchange ->
            Response.status(500, "{\"message\":123,\"error\":456}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertTrue(error.getMessage().contains("123"));
        }
    }
}
