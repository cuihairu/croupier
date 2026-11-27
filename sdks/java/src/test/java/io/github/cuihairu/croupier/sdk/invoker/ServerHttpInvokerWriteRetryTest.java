package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.InputStream;
import java.net.InetAddress;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * AUD-068: a write may have taken effect before its response was lost, so non-GET
 * requests are single-shot unless the caller opts in via {@code retryWrites}, and
 * the final write failure of unknown outcome is marked {@code RESULT_UNKNOWN}.
 * Reads keep the previous retry behavior and are never marked result-unknown.
 */
class ServerHttpInvokerWriteRetryTest {

    private static RetryConfig fastRetry() {
        return RetryConfig.builder().enabled(true).maxAttempts(3).initialDelayMs(1).maxDelayMs(2).jitterFactor(0).build();
    }
    private static RetryConfig fastRetry(boolean retryWrites) {
        return RetryConfig.builder().enabled(true).maxAttempts(3).initialDelayMs(1).maxDelayMs(2).jitterFactor(0).retryWrites(retryWrites).build();
    }

    @Test
    void writeIsNotRetriedByDefaultAndMarkedResultUnknown() throws Exception {
        // Raw server: consume the POST, then drop the connection without a response.
        AtomicInteger connections = new AtomicInteger();
        try (DropServer server = new DropServer(connections)) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address("http://127.0.0.1:" + server.port()).retry(fastRetry()).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.RESULT_UNKNOWN, error.getErrorCode());
            assertEquals(1, connections.get(), "the write must not be replayed after its side effect");
        }
    }

    @Test
    void writeWithRetryable503IsSingleShotByDefault() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            calls.incrementAndGet();
            return new MockResponse(503, "{\"message\":\"temporarily unavailable\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry()).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.RESULT_UNKNOWN, error.getErrorCode());
            assertEquals(503, server.lastStatus());
            assertEquals(1, calls.get(), "retryable 5xx must not replay the write by default");
        }
    }

    @Test
    void retryWritesOptsInvokeBackIntoRetries() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            boolean first = calls.incrementAndGet() < 2;
            return first ? new MockResponse(503, "{\"message\":\"unavailable\"}")
                : new MockResponse(200, "{\"result\":{\"ok\":true}}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry(true)).build());
            assertEquals("{\"ok\":true}", invoker.invoke("player.ban", "{}"));
            assertEquals(2, calls.get());
        }
    }

    @Test
    void retryWritesOptsStartTaskBackIntoRetries() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            boolean first = calls.incrementAndGet() < 2;
            return first ? new MockResponse(503, "{\"message\":\"unavailable\"}")
                : new MockResponse(200, "{\"taskId\":\"task-1\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry(true)).build());
            assertEquals("task-1", invoker.startTask("report.generate", "{}"));
            assertEquals(2, calls.get());
        }
    }

    @Test
    void exhaustedWriteRetryIsMarkedResultUnknown() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            calls.incrementAndGet();
            return new MockResponse(503, "{\"message\":\"unavailable\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry(true)).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.RESULT_UNKNOWN, error.getErrorCode());
            assertEquals(3, calls.get());
        }
    }

    @Test
    void explicitRejectionIsNeitherRetriedNorMarkedUnknown() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            calls.incrementAndGet();
            return new MockResponse(403, "{\"message\":\"forbidden\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry(true)).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("player.ban", "{}"));
            assertEquals(InvokerException.ErrorCode.PERMISSION_DENIED, error.getErrorCode());
            assertEquals(1, calls.get(), "explicit rejection is final");
        }
    }

    @Test
    void getKeepsRetryingAndReadFailuresAreNotMarkedUnknown() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            boolean success = calls.incrementAndGet() >= 3;
            return success
                ? new MockResponse(200, "{\"id\":\"task-1\",\"functionId\":\"report.generate\",\"status\":\"running\"}")
                : new MockResponse(503, "{\"message\":\"unavailable\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry()).build());
            TaskStatusInfo status = invoker.getTaskStatus("task-1");
            assertEquals("running", status.status());
            assertEquals(3, calls.get(), "reads keep the default retry behavior");
        }

        AtomicInteger failedCalls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            failedCalls.incrementAndGet();
            return new MockResponse(503, "{\"message\":\"unavailable\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry()).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.getTaskStatus("task-1"));
            assertEquals(3, failedCalls.get());
            assertNotEquals(InvokerException.ErrorCode.RESULT_UNKNOWN, error.getErrorCode(), "read failures are never result-unknown");
        }
    }

    @Test
    void cancelTaskWriteIsSingleShotByDefault() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(exchange -> {
            calls.incrementAndGet();
            return new MockResponse(503, "{\"message\":\"unavailable\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(fastRetry()).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.cancelTask("task-1"));
            assertEquals(InvokerException.ErrorCode.RESULT_UNKNOWN, error.getErrorCode());
            assertEquals(1, calls.get());
        }
    }

    private record MockResponse(int status, String body) { }

    @FunctionalInterface
    private interface MockResponder { MockResponse respond(HttpExchange exchange) throws IOException; }

    private static final class MockServer implements AutoCloseable {
        private final HttpServer server;
        private final AtomicInteger lastStatus = new AtomicInteger();
        MockServer(MockResponder responder) throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                MockResponse response = responder.respond(exchange);
                lastStatus.set(response.status());
                byte[] body = response.body().getBytes(StandardCharsets.UTF_8);
                exchange.getResponseHeaders().add("Content-Type", "application/json");
                exchange.sendResponseHeaders(response.status(), body.length);
                exchange.getResponseBody().write(body);
                exchange.close();
            });
            server.start();
        }
        String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
        int lastStatus() { return lastStatus.get(); }
        @Override public void close() { server.stop(0); }
    }

    /** Accepts requests in a loop, consumes each fully, then drops the connection without responding. */
    private static final class DropServer implements AutoCloseable {
        private final ServerSocket socket;
        private final Thread thread;
        DropServer(AtomicInteger connections) throws IOException {
            socket = new ServerSocket(0, 8, InetAddress.getByName("127.0.0.1"));
            thread = new Thread(() -> {
                while (!socket.isClosed()) {
                    try (Socket connection = socket.accept()) {
                        connections.incrementAndGet();
                        InputStream input = connection.getInputStream();
                        StringBuilder headers = new StringBuilder();
                        int contentLength = 0;
                        int b;
                        while ((b = input.read()) != -1) {
                            headers.append((char) b);
                            if (headers.length() >= 4 && headers.substring(headers.length() - 4).equals("\r\n\r\n")) break;
                        }
                        for (String line : headers.toString().split("\r\n")) {
                            if (line.toLowerCase().startsWith("content-length:")) {
                                contentLength = Integer.parseInt(line.substring(15).trim());
                            }
                        }
                        for (int i = 0; i < contentLength; i++) input.read();
                    } catch (IOException ignored) {
                        // socket closed or peer aborted; keep accepting until shutdown
                    }
                }
            });
            thread.setDaemon(true);
            thread.start();
        }
        int port() { return socket.getLocalPort(); }
        @Override public void close() throws IOException { socket.close(); }
    }
}
