package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Function;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Second coverage boost: full task lifecycle through the Server HTTP invoker,
 * including streamTask reactive semantics and getTaskStatus field mapping.
 */
@DisplayName("ServerHttpInvoker task lifecycle boost")
class ServerHttpInvokerTaskBoostTest {

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

    private static ServerHttpInvoker invoker(String address, RetryConfig retry) {
        InvokerConfig.Builder builder = InvokerConfig.builder().address(address);
        if (retry != null) {
            builder.retry(retry);
        }
        return new ServerHttpInvoker(builder.build());
    }

    @Test
    @DisplayName("getTaskStatus maps every server field")
    void getTaskStatusFieldMapping() throws Exception {
        try (MockServer server = new MockServer(ex -> Response.ok(
            "{\"id\":\"t-7\",\"functionId\":\"report.generate\",\"status\":\"running\","
                + "\"progress\":42,\"message\":\"halfway\",\"error\":null,\"result\":{\"partial\":true},"
                + "\"startedAt\":\"2026-01-01T00:00:00Z\",\"finishedAt\":null,"
                + "\"createdAt\":\"2025-12-31T23:00:00Z\",\"updatedAt\":\"2026-01-01T00:10:00Z\"}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            try {
                TaskStatusInfo status = invoker.getTaskStatus("t-7");
                assertEquals("t-7", status.taskId());
                assertEquals("report.generate", status.functionId());
                assertEquals("running", status.status());
                assertEquals(42, status.progress());
                assertEquals("halfway", status.message());
                assertTrue(status.result().contains("partial"));
                assertNull(status.error());
                assertEquals("2026-01-01T00:00:00Z", status.startedAt());
                assertNull(status.finishedAt());
                assertNotNull(status.createdAt());
                assertNotNull(status.updatedAt());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("getTaskStatus keeps the requested id and unknown status defaults")
    void getTaskStatusFallbackId() throws Exception {
        try (MockServer server = new MockServer(ex -> Response.ok("{}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            try {
                TaskStatusInfo status = invoker.getTaskStatus("requested-id");
                assertEquals("requested-id", status.taskId());
                assertEquals("unknown", status.status());
                assertNull(status.progress());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("streamTask emits events until a terminal event arrives")
    void streamTaskTerminalEvent() throws Exception {
        try (MockServer server = new MockServer(ex -> Response.ok(
            "{\"items\":["
                + "{\"seq\":1,\"type\":\"progress\",\"progress\":25,\"message\":\"quarter\"},"
                + "{\"seq\":2,\"type\":\"log\",\"message\":\"working\"},"
                + "{\"seq\":3,\"type\":\"failed\",\"message\":\"disk full\"}"
                + "],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            try {
                List<TaskEventInfo> events = subscribeAndAwait(invoker.streamTask("t-1"));

                assertEquals(3, events.size());
                assertEquals(25, events.get(0).getProgress());
                assertEquals("working", events.get(1).getMessage());
                TaskEventInfo failure = events.get(2);
                assertEquals("failed", failure.getType());
                assertEquals("disk full", failure.getError());
                assertTrue(failure.isDone());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("streamTask maps done events to completed")
    void streamTaskDoneMapsToCompleted() throws Exception {
        try (MockServer server = new MockServer(ex -> Response.ok(
            "{\"items\":[{\"seq\":1,\"type\":\"done\"}],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            try {
                List<TaskEventInfo> events = subscribeAndAwait(invoker.streamTask("t-2"));
                assertEquals(1, events.size());
                assertEquals("completed", events.get(0).getType());
                assertTrue(events.get(0).isDone());
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("startTask sends gameId/env scope headers from config")
    void startTaskScopedHeaders() throws Exception {
        final List<String> seenGameIds = new CopyOnWriteArrayList<>();
        try (MockServer server = new MockServer(ex -> {
            seenGameIds.add(ex.getRequestHeaders().getFirst("X-Game-ID"));
            return Response.ok("{\"taskId\":\"t-3\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl())
                .gameId("scoped-game")
                .env("production")
                .build());
            try {
                assertEquals("t-3", invoker.startTask("fn", "{}"));
                assertEquals("scoped-game", seenGameIds.get(0));
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("cancelTask succeeds and reports failures")
    void cancelTaskSemantics() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(ex -> {
            calls.incrementAndGet();
            return Response.ok("{\"message\":\"accepted\"}");
        })) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            try {
                assertDoesNotThrow(() -> invoker.cancelTask("t-4"));
                assertEquals(1, calls.get());
            } finally {
                invoker.close();
            }
        }

        try (MockServer server = new MockServer(ex ->
                Response.status(409, "{\"message\":\"already finished\"}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            try {
                InvokerException error = assertThrows(InvokerException.class,
                    () -> invoker.cancelTask("t-4"));
                assertTrue(error.getMessage().contains("already finished"));
            } finally {
                invoker.close();
            }
        }
    }

    @Test
    @DisplayName("request-level retry overrides config for tasks too")
    void taskRetryOverride() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(ex -> {
            calls.incrementAndGet();
            return Response.status(503, "{\"message\":\"down\"}");
        })) {
            RetryConfig globalRetry = RetryConfig.builder().maxAttempts(5).initialDelayMs(1).build();
            RetryConfig disabled = RetryConfig.builder().enabled(false).build();
            ServerHttpInvoker invoker = invoker(server.baseUrl(), globalRetry);
            try {
                assertThrows(InvokerException.class,
                    () -> invoker.startTask("fn", "{}", InvokeOptions.builder().retry(disabled).build()));
                assertEquals(1, calls.get());
            } finally {
                invoker.close();
            }
        }
    }

    /** Drives a reactive stream into a plain list with a bounded wait. */
    private static List<TaskEventInfo> subscribeAndAwait(
        org.reactivestreams.Publisher<TaskEventInfo> publisher) throws Exception {
        List<TaskEventInfo> events = new CopyOnWriteArrayList<>();
        CountDownLatch completed = new CountDownLatch(1);
        AtomicInteger failures = new AtomicInteger();
        publisher.subscribe(new Subscriber<>() {
            @Override public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }
            @Override public void onNext(TaskEventInfo event) {
                events.add(event);
            }
            @Override public void onError(Throwable error) {
                failures.incrementAndGet();
                completed.countDown();
            }
            @Override public void onComplete() {
                completed.countDown();
            }
        });
        assertTrue(completed.await(10, TimeUnit.SECONDS), "stream did not complete in time");
        assertEquals(0, failures.get(), "stream completed with an error");
        return events;
    }
}
