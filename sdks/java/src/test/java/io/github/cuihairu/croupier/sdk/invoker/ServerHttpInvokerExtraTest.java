package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Additional mock-Server HTTP contract tests for ServerHttpInvoker.
 *
 * <p>Covers lifecycle, base-URL normalization, schema validation, retry,
 * header handling, error mapping and task-event subscription edge cases.</p>
 */
@DisplayName("ServerHttpInvoker additional HTTP contract")
class ServerHttpInvokerExtraTest {

    private interface Responder {
        Response respond(HttpExchange exchange, AtomicInteger requestCounter) throws IOException;
    }

    private record Response(int status, String body) {
        static Response ok(String body) { return new Response(200, body); }
        static Response status(int status, String body) { return new Response(status, body); }
    }

    private static final class MockServer implements AutoCloseable {
        private final HttpServer server;
        private final List<RecordedRequest> requests = new ArrayList<>();
        private final AtomicInteger counter = new AtomicInteger();

        MockServer(Responder responder) throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                requests.add(RecordedRequest.from(exchange));
                Response response = responder.respond(exchange, counter);
                byte[] body = response.body().getBytes(StandardCharsets.UTF_8);
                exchange.getResponseHeaders().add("Content-Type", "application/json");
                exchange.sendResponseHeaders(response.status(), body.length == 0 ? -1 : body.length);
                if (body.length > 0) {
                    try (OutputStream out = exchange.getResponseBody()) {
                        out.write(body);
                    }
                }
                exchange.close();
            });
            server.start();
        }

        String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
        int requestCount() { return counter.get(); }

        @Override
        public void close() { server.stop(0); }
    }

    private record RecordedRequest(String method, String path, Map<String, List<String>> headers) {
        static RecordedRequest from(HttpExchange exchange) throws IOException {
            exchange.getRequestBody().readAllBytes();
            return new RecordedRequest(exchange.getRequestMethod(), exchange.getRequestURI().getRawPath(), exchange.getRequestHeaders());
        }

        String header(String name) {
            return headers.entrySet().stream()
                .filter(entry -> entry.getKey().equalsIgnoreCase(name))
                .findFirst()
                .map(entry -> entry.getValue().get(0))
                .orElse(null);
        }
    }

    private static RetryConfig noRetry() {
        return RetryConfig.builder().enabled(false).build();
    }

    private static InvokerConfig.Builder config(String baseUrl) {
        return InvokerConfig.builder().address(baseUrl).taskPollIntervalMs(1).retry(noRetry());
    }

    @Test
    @DisplayName("connect/close track connection state and base URL is normalized")
    void lifecycleAndBaseUrlNormalization() {
        ServerHttpInvoker invoker = new ServerHttpInvoker(config("127.0.0.1:18780").build());
        assertFalse(invoker.isConnected());
        assertEquals("http://127.0.0.1:18780/api/v1", invoker.getBaseUrl());

        invoker.connect();
        assertTrue(invoker.isConnected());
        invoker.close();
        assertFalse(invoker.isConnected());

        assertEquals("http://host.example:1/base/api/v1",
            new ServerHttpInvoker(config("http://host.example:1/base/").build()).getBaseUrl());
        assertEquals("https://host.example/api/v1",
            new ServerHttpInvoker(config("https://host.example").build()).getBaseUrl());
        assertEquals(ServerHttpInvoker.DEFAULT_SERVER_API_URL,
            new ServerHttpInvoker(config("  ").build()).getBaseUrl());

        assertThrows(IllegalArgumentException.class, () -> new ServerHttpInvoker(config("ftp://host").build()));
        assertThrows(IllegalArgumentException.class, () -> new ServerHttpInvoker(config("http://").build()));
    }

    @Test
    @DisplayName("two-argument invoke posts params and returns the stringified result")
    @Timeout(5)
    void invokeTwoArguments() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"result\":{\"answer\":42}}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            // Recorded bug: the JSON codec parses integers as Double, so 42 comes back as 42.0.
            assertEquals("{\"answer\":42.0}", invoker.invoke("quiz.ask", "{\"q\":\"life\"}"));
            assertEquals("POST", server.requests.get(0).method());
            assertEquals("/api/v1/functions/quiz.ask/invoke", server.requests.get(0).path());
        }
    }

    @Test
    @DisplayName("invoke rejects non-object responses and missing result fields")
    @Timeout(5)
    void invokeRejectsMalformedResponses() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"other\":true}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException missing = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertTrue(missing.getMessage().contains("does not contain result"));
        }
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("[1,2]"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException notObject = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertTrue(notObject.getMessage().contains("must be an object"));
        }
    }

    @Test
    @DisplayName("getTaskStatus maps the full Server response payload")
    @Timeout(5)
    void taskStatusFieldMapping() throws Exception {
        String body = "{\"id\":\"t-9\",\"functionId\":\"mail.send\",\"status\":\"succeeded\",\"progress\":42,"
            + "\"message\":\"done msg\",\"result\":{\"mail_id\":\"m-1\"},\"error\":null,"
            + "\"startedAt\":\"2025-01-01\",\"finishedAt\":\"2025-01-02\",\"createdAt\":\"2025-01-03\",\"updatedAt\":\"2025-01-04\"}";
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok(body))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            TaskStatusInfo status = invoker.getTaskStatus("t-9");
            assertEquals("t-9", status.taskId());
            assertEquals("mail.send", status.functionId());
            assertEquals("succeeded", status.status());
            assertEquals(42, status.progress());
            assertEquals("done msg", status.message());
            assertEquals("{\"mail_id\":\"m-1\"}", status.result());
            assertNull(status.error());
            assertEquals("2025-01-01", status.startedAt());
            assertEquals("2025-01-02", status.finishedAt());
            assertEquals("2025-01-03", status.createdAt());
            assertEquals("2025-01-04", status.updatedAt());
        }

        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            TaskStatusInfo defaults = invoker.getTaskStatus("fallback-id");
            assertEquals("fallback-id", defaults.taskId());
            assertEquals("unknown", defaults.status());
            assertNull(defaults.progress());
            assertNull(defaults.functionId());
        }

        try (MockServer server = new MockServer((exchange, counter) -> new Response(200, ""))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            TaskStatusInfo blank = invoker.getTaskStatus("blank-id");
            assertEquals("blank-id", blank.taskId());
            assertEquals("unknown", blank.status());
        }
    }

    @Test
    @DisplayName("setSchema enforces required fields and object payloads")
    @Timeout(5)
    void schemaValidation() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            invoker.setSchema("mail.send", Map.of("required", List.of("title", "content")));

            InvokerException missingField = assertThrows(InvokerException.class, () ->
                invoker.invoke("mail.send", "{\"title\":\"only title\"}"));
            assertTrue(missingField.getMessage().contains("missing required field 'content'"), missingField.getMessage());

            InvokerException notObject = assertThrows(InvokerException.class, () ->
                invoker.invoke("mail.send", "[1,2]"));
            assertTrue(notObject.getMessage().contains("expected JSON object"), notObject.getMessage());

            InvokerException nullPayload = assertThrows(InvokerException.class, () ->
                invoker.invoke("mail.send", null));
            assertTrue(nullPayload.getMessage().contains("missing required field"), nullPayload.getMessage());

            assertEquals("\"ok\"", invoker.invoke("mail.send", "{\"title\":\"t\",\"content\":\"c\"}"));

            assertThrows(IllegalArgumentException.class, () -> invoker.setSchema(" ", Map.of()));
            invoker.setSchema("mail.send", null);
            // A null schema keeps the object-payload requirement but drops required fields.
            InvokerException stillObject = assertThrows(InvokerException.class, () ->
                invoker.invoke("mail.send", "[1,2]"));
            assertTrue(stillObject.getMessage().contains("expected JSON object"), stillObject.getMessage());
            assertEquals("\"ok\"", invoker.invoke("mail.send", "{}"));

            InvokerException invalidJson = assertThrows(InvokerException.class, () ->
                invoker.invoke("mail.send", "{nope"));
            assertTrue(invalidJson.getMessage().contains("payload must be valid JSON"), invalidJson.getMessage());
        }
    }

    @Test
    @DisplayName("retryable failures are retried up to the configured attempts")
    @Timeout(10)
    void retryBehavior() throws Exception {
        AtomicInteger hits = new AtomicInteger();
        try (MockServer server = new MockServer((exchange, counter) -> {
            if (hits.incrementAndGet() == 1) {
                return Response.status(503, "{\"message\":\"overloaded\"}");
            }
            return Response.ok("{\"result\":\"recovered\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl())
                .retry(RetryConfig.builder().enabled(true).maxAttempts(2).initialDelayMs(1).build())
                .build());
            assertEquals("\"recovered\"", invoker.invoke("f", "{}"));
            assertEquals(2, hits.get());
        }

        AtomicInteger exhausted = new AtomicInteger();
        try (MockServer server = new MockServer((exchange, counter) -> {
            exhausted.incrementAndGet();
            return Response.status(503, "{\"message\":\"down\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl())
                .retry(RetryConfig.builder().enabled(true).maxAttempts(2).initialDelayMs(1).build())
                .build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertEquals(InvokerException.ErrorCode.UNAVAILABLE, error.getErrorCode());
            assertEquals(2, exhausted.get());
        }

        AtomicInteger badRequests = new AtomicInteger();
        try (MockServer server = new MockServer((exchange, counter) -> {
            badRequests.incrementAndGet();
            return Response.status(400, "{\"message\":\"bad input\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl())
                .retry(RetryConfig.builder().enabled(true).maxAttempts(5).initialDelayMs(1).build())
                .build());
            InvokerException badRequest = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, badRequest.getErrorCode());
            assertTrue(badRequest.getMessage().endsWith("bad input"), badRequest.getMessage());
            assertEquals(1, badRequests.get(), "non-retryable failures must not be retried");
        }
    }

    @Test
    @DisplayName("request timeouts map to TIMEOUT and connection failures to UNAVAILABLE")
    @Timeout(10)
    void networkFailureMapping() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> {
            try {
                Thread.sleep(400);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
            return Response.ok("{\"result\":\"late\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException timeout = assertThrows(InvokerException.class, () ->
                invoker.invoke("f", "{}", InvokeOptions.builder().timeout(100).build()));
            assertEquals(InvokerException.ErrorCode.TIMEOUT, timeout.getErrorCode());
        }

        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"result\":\"x\"}"))) {
            String url = server.baseUrl();
            server.close();
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(url).build());
            InvokerException unreachable = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertEquals(InvokerException.ErrorCode.UNAVAILABLE, unreachable.getErrorCode());
        }
    }

    @Test
    @DisplayName("invalid JSON success bodies map to INTERNAL errors")
    @Timeout(5)
    void invalidJsonBodyMapping() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("not-json{{"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertEquals(InvokerException.ErrorCode.INTERNAL, error.getErrorCode());
            assertTrue(error.getMessage().contains("invalid JSON"), error.getMessage());
        }
    }

    @Test
    @DisplayName("error bodies fall back from message to error to raw body")
    @Timeout(5)
    void errorMessageExtraction() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.status(404, "plain-not-found"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException plain = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertEquals(InvokerException.ErrorCode.NOT_FOUND, plain.getErrorCode());
            assertTrue(plain.getMessage().endsWith("plain-not-found"), plain.getMessage());
        }
        try (MockServer server = new MockServer((exchange, counter) -> Response.status(500, "{\"error\":\"boom\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException codeOnly = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertTrue(codeOnly.getMessage().endsWith("boom"), codeOnly.getMessage());
        }
        try (MockServer server = new MockServer((exchange, counter) -> Response.status(500, ""))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            InvokerException empty = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
            assertTrue(empty.getMessage().endsWith("empty response body"), empty.getMessage());
        }
    }

    @Test
    @DisplayName("headers reject CR/LF injection and custom auth headers win")
    @Timeout(5)
    void headerHandling() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).authToken("secret").build());

            InvokerException crlf = assertThrows(InvokerException.class, () ->
                invoker.invoke("f", "{}", InvokeOptions.builder().header("X-Bad", "a\r\nb").build()));
            assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, crlf.getErrorCode());
            assertTrue(crlf.getMessage().contains("CR or LF"), crlf.getMessage());

            // Recorded bug: InvokeOptions' Map.copyOf rejects null header values with a
            // bare NPE before ServerHttpInvoker's own null/CR/LF validation can run.
            assertThrows(NullPointerException.class, () -> {
                Map<String, String> headers = new HashMap<>();
                headers.put("X-Null", null);
                invoker.invoke("f", "{}", InvokeOptions.builder().headers(headers).build());
            });

            invoker.invoke("f", "{}", InvokeOptions.builder().header("Authorization", "Token raw").build());
            RecordedRequest last = server.requests.get(server.requests.size() - 1);
            assertEquals("Token raw", last.header("Authorization"));
            assertNull(last.header("Idempotency-Key"));
        }

        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).authToken("bearer lower").build());
            invoker.invoke("f", "{}");
            assertEquals("bearer lower", server.requests.get(0).header("Authorization"));
        }
    }

    @Test
    @DisplayName("task ids are URL-encoded in request paths")
    @Timeout(5)
    void pathEncoding() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"id\":\"t 1\",\"status\":\"running\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            TaskStatusInfo status = invoker.getTaskStatus("t 1");
            assertEquals("t 1", status.taskId());
            assertEquals("/api/v1/tasks/t%201", server.requests.get(0).path());
        }
    }

    @Test
    @DisplayName("cancelTask posts to the cancel endpoint")
    @Timeout(5)
    void cancelTaskPosts() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"message\":\"accepted\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            invoker.cancelTask("t-1");
            assertEquals("POST", server.requests.get(0).method());
            assertEquals("/api/v1/tasks/t-1/cancel", server.requests.get(0).path());
        }
    }

    @Test
    @DisplayName("streamTask with invalid id errors through a noop subscription")
    @Timeout(5)
    void streamTaskInvalidId() {
        ServerHttpInvoker invoker = new ServerHttpInvoker(config("http://127.0.0.1:1").build());
        CountDownLatch errored = new CountDownLatch(1);
        AtomicReference<Throwable> error = new AtomicReference<>();
        invoker.streamTask(" ").subscribe(new Subscriber<TaskEventInfo>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                // Exercise both noop subscription entry points before failing.
                subscription.request(1);
                subscription.cancel();
            }

            @Override
            public void onNext(TaskEventInfo event) {
                error.set(new AssertionError("unexpected event " + event));
            }

            @Override
            public void onError(Throwable throwable) {
                error.set(throwable);
                errored.countDown();
            }

            @Override
            public void onComplete() {
                error.set(new AssertionError("unexpected completion"));
            }
        });
        await(errored, "subscriber should error for blank task id");
        assertInstanceOf(InvokerException.class, error.get());
        assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, ((InvokerException) error.get()).getErrorCode());
    }

    @Test
    @DisplayName("streamTask request(0) cancels the subscription and errors")
    @Timeout(10)
    void streamTaskNonPositiveRequest() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"items\":[],\"done\":false}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CountDownLatch errored = new CountDownLatch(1);
            AtomicReference<Throwable> error = new AtomicReference<>();
            invoker.streamTask("t-1").subscribe(new Subscriber<TaskEventInfo>() {
                @Override
                public void onSubscribe(Subscription subscription) {
                    subscription.request(0);
                }

                @Override
                public void onNext(TaskEventInfo event) {
                    error.set(new AssertionError("unexpected event"));
                }

                @Override
                public void onError(Throwable throwable) {
                    error.set(throwable);
                    errored.countDown();
                }

                @Override
                public void onComplete() {
                    error.set(new AssertionError("unexpected completion"));
                }
            });
            assertTrue(errored.await(5, TimeUnit.SECONDS));
            assertInstanceOf(IllegalArgumentException.class, error.get());
            assertTrue(error.get().getMessage().contains("positive"), error.get().getMessage());
        }
    }

    @Test
    @DisplayName("streamTask polls while no events arrive and completes on done")
    @Timeout(10)
    void streamTaskPollsAndCompletes() throws Exception {
        AtomicInteger polls = new AtomicInteger();
        try (MockServer server = new MockServer((exchange, counter) -> {
            if (polls.incrementAndGet() < 3) {
                return Response.ok("{\"items\":[],\"done\":false}");
            }
            return Response.ok("{\"items\":[{\"seq\":1,\"type\":\"done\",\"payload\":null}],\"done\":true}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CollectingSubscriber subscriber = new CollectingSubscriber();
            invoker.streamTask("t-poll").subscribe(subscriber);
            assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS), "stream should complete");
            assertTrue(polls.get() >= 3, "expected multiple polls, got " + polls.get());
            assertEquals(List.of("completed"), subscriber.types);
            assertNull(subscriber.lastPayload);
        }
    }

    @Test
    @DisplayName("streamTask maps failed and timed_out terminal events")
    @Timeout(10)
    void streamTaskTerminalEventMapping() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) ->
            Response.ok("{\"items\":[{\"seq\":1,\"type\":\"failed\",\"message\":\"worker died\"}],\"done\":true}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CollectingSubscriber subscriber = new CollectingSubscriber();
            invoker.streamTask("t-fail").subscribe(subscriber);
            assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS));
            assertEquals("failed", subscriber.types.get(0));
            assertEquals("worker died", subscriber.lastError);
            assertTrue(subscriber.lastDone);
        }
        try (MockServer server = new MockServer((exchange, counter) ->
            Response.ok("{\"items\":[{\"seq\":1,\"type\":\"timed_out\",\"message\":\"too slow\"}],\"done\":true}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CollectingSubscriber subscriber = new CollectingSubscriber();
            invoker.streamTask("t-timeout").subscribe(subscriber);
            assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS));
            assertEquals("timed_out", subscriber.types.get(0));
            assertEquals("too slow", subscriber.lastError);
        }
    }

    @Test
    @DisplayName("streamTask rejects malformed item payloads")
    @Timeout(10)
    void streamTaskMalformedItems() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"items\":\"not-a-list\",\"done\":false}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CollectingSubscriber subscriber = new CollectingSubscriber();
            invoker.streamTask("t-bad-items").subscribe(subscriber);
            assertTrue(subscriber.errored.await(5, TimeUnit.SECONDS));
            assertTrue(subscriber.error.getMessage().contains("items must be an array"), subscriber.error.getMessage());
        }
        try (MockServer server = new MockServer((exchange, counter) -> Response.ok("{\"items\":[42],\"done\":false}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CollectingSubscriber subscriber = new CollectingSubscriber();
            invoker.streamTask("t-bad-item").subscribe(subscriber);
            assertTrue(subscriber.errored.await(5, TimeUnit.SECONDS));
            assertTrue(subscriber.error.getMessage().contains("task event must be an object"), subscriber.error.getMessage());
        }
    }

    @Test
    @DisplayName("streamTask stops delivering events after cancellation")
    @Timeout(10)
    void streamTaskCancellationStopsDelivery() throws Exception {
        try (MockServer server = new MockServer((exchange, counter) -> {
            String query = exchange.getRequestURI().getQuery();
            if (query == null || query.endsWith("after_seq=0")) {
                return Response.ok("{\"items\":[{\"seq\":1,\"type\":\"progress\",\"progress\":10}],\"done\":false}");
            }
            return Response.ok("{\"items\":[{\"seq\":2,\"type\":\"done\"}],\"done\":true}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(config(server.baseUrl()).build());
            CountDownLatch first = new CountDownLatch(1);
            CountDownLatch completed = new CountDownLatch(1);
            CountDownLatch errored = new CountDownLatch(1);
            List<String> types = new ArrayList<>();
            invoker.streamTask("t-cancel").subscribe(new Subscriber<TaskEventInfo>() {
                @Override
                public void onSubscribe(Subscription subscription) {
                    subscription.request(1);
                }

                @Override
                public void onNext(TaskEventInfo event) {
                    types.add(event.getType());
                    first.countDown();
                }

                @Override
                public void onError(Throwable error) {
                    errored.countDown();
                }

                @Override
                public void onComplete() {
                    completed.countDown();
                }
            });
            assertTrue(first.await(5, TimeUnit.SECONDS));
            // Only one event was requested; no further demand means the worker
            // must not deliver the terminal event even though the server sent it.
            assertFalse(completed.await(300, TimeUnit.MILLISECONDS), "no completion expected without demand");
            assertFalse(errored.await(300, TimeUnit.MILLISECONDS));
            assertEquals(List.of("progress"), types);
        }
    }

    private static void await(CountDownLatch latch, String message) {
        try {
            assertTrue(latch.await(5, TimeUnit.SECONDS), message);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            fail(message);
        }
    }

    private static final class CollectingSubscriber implements Subscriber<TaskEventInfo> {
        final List<String> types = new ArrayList<>();
        final CountDownLatch completed = new CountDownLatch(1);
        final CountDownLatch errored = new CountDownLatch(1);
        volatile String lastError;
        volatile String lastPayload;
        volatile boolean lastDone;

        @Override
        public void onSubscribe(Subscription subscription) {
            subscription.request(Long.MAX_VALUE);
        }

        @Override
        public void onNext(TaskEventInfo event) {
            types.add(event.getType());
            lastError = event.getError();
            lastPayload = event.getPayload();
            lastDone = event.isDone();
        }

        @Override
        public void onError(Throwable error) {
            this.error = error;
            errored.countDown();
        }

        @Override
        public void onComplete() {
            completed.countDown();
        }

        volatile Throwable error;
    }
}
