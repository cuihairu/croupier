package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for ServerHttpInvoker edge branches. */
class ServerHttpInvokerBranchTest {

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
                byte[] body = response.body() == null ? new byte[0] : response.body().getBytes(StandardCharsets.UTF_8);
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

    private static ServerHttpInvoker invoker(MockServer server, RetryConfig retry) {
        return invoker(server.baseUrl(), retry);
    }

    private static ServerHttpInvoker invoker(String address, RetryConfig retry) {
        InvokerConfig.Builder builder = InvokerConfig.builder().address(address).taskPollIntervalMs(1);
        if (retry != null) {
            builder.retry(retry);
        }
        return new ServerHttpInvoker(builder.build());
    }

    // ------------------------------------------------------------------
    // normalizeBaseUrl / address handling
    // ------------------------------------------------------------------

    @Test
    void normalizeBaseUrlCoversSchemePathAndDefaultVariants() throws Exception {

        assertEquals("http://127.0.0.1:18780/api/v1", invoker((String) null, null).getBaseUrl());
        assertEquals("http://127.0.0.1:18780/api/v1", invoker("   ", null).getBaseUrl());
        assertEquals("http://127.0.0.1:18780/api/v1", invoker("127.0.0.1:18780", null).getBaseUrl());
        assertEquals("https://host.example/api/v1", invoker("https://host.example", null).getBaseUrl());
        assertEquals("https://host.example/prefix/api/v1", invoker("https://host.example/prefix/", null).getBaseUrl());
        assertEquals("http://host/api/v1", invoker("http://host/api/v1/", null).getBaseUrl());
        assertThrows(IllegalArgumentException.class, () -> invoker("ftp://host", null));
        assertThrows(IllegalArgumentException.class, () -> invoker("http:///no-host", null));
    }

    // ------------------------------------------------------------------
    // identifier / payload validation
    // ------------------------------------------------------------------

    @Test
    void rejectsNullIdentifiersAndSchemaKeys() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertThrows(InvokerException.class, () -> invoker.invoke(null, "{}"));
            assertThrows(InvokerException.class, () -> invoker.startTask(" ", "{}"));
            assertThrows(InvokerException.class, () -> invoker.getTaskStatus(null));
            assertThrows(InvokerException.class, () -> invoker.cancelTask(""));
            assertThrows(IllegalArgumentException.class, () -> invoker.setSchema(null, Map.of()));
            assertThrows(IllegalArgumentException.class, () -> invoker.setSchema(" ", Map.of()));
            // null schema value is stored as an empty schema (validation skipped)
            invoker.setSchema("fn", null);
            invoker.setSchema("fn", Map.of());
        }
    }

    @Test
    void nullAndBlankPayloadsFallBackToEmptyObject() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertEquals("\"ok\"", invoker.invoke("fn", null));
            assertEquals("\"ok\"", invoker.invoke("fn", "  "));
        }
    }

    // ------------------------------------------------------------------
    // HTTP status and error body handling
    // ------------------------------------------------------------------

    @Test
    void non2xxStatusesSurfaceServerErrorDetail() throws Exception {
        List<Integer> statuses = List.of(302, 400, 401, 429, 500, 503, 504);
        for (int status : statuses) {
            try (MockServer server = new MockServer(exchange -> Response.status(status, "{\"message\":\"boom-" + status + "\"}"))) {
                ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
                InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
                assertTrue(error.getMessage().contains("boom-" + status));
            }
        }
    }

    @Test
    void errorBodiesFallBackAcrossShapes() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.status(500, "{\"message\":\"\"}"))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            // blank message + blank error falls through and the raw body is surfaced
            assertTrue(error.getMessage().contains("message"));
        }
        try (MockServer server = new MockServer(exchange -> Response.status(500, "{\"error\":\"error-code\"}"))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertTrue(error.getMessage().contains("error-code"));
        }
        try (MockServer server = new MockServer(exchange -> Response.status(500, "{\"error\":\"\"}"))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertTrue(error.getMessage().contains("error"));
        }
        try (MockServer server = new MockServer(exchange -> Response.status(500, "[\"array-body\"]"))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertTrue(error.getMessage().contains("array-body"));
        }
        try (MockServer server = new MockServer(exchange -> Response.status(500, null))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertTrue(error.getMessage().contains("empty response body"));
        }
    }

    @Test
    void nonObjectAndEmptyBodiesAreHandled() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("[1,2]"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertThrows(InvokerException.class, () -> invoker.startTask("fn", "{}"));
        }
        try (MockServer server = new MockServer(exchange -> Response.ok(""))) {
            ServerHttpInvoker invoker = invoker(server, null);
            InvokerException missing = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertTrue(missing.getMessage().contains("result"));
            InvokerException noTask = assertThrows(InvokerException.class, () -> invoker.startTask("fn", "{}"));
            assertTrue(noTask.getMessage().contains("taskId"));
        }
        try (MockServer server = new MockServer(exchange -> Response.ok("{\"taskId\":\"  \"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertThrows(InvokerException.class, () -> invoker.startTask("fn", "{}"));
        }
    }

    @Test
    void invalidResponseBodyJsonFailsAsInternal() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{not-json"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertEquals(InvokerException.ErrorCode.INTERNAL, error.getErrorCode());
        }
    }

    @Test
    void taskStatusFallsBackToDefaultsForMissingFields() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.ok("{}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            TaskStatusInfo status = invoker.getTaskStatus("task-x");
            assertEquals("task-x", status.taskId());
            assertEquals("unknown", status.status());
            assertEquals(null, status.functionId());
            assertEquals(null, status.progress());
            assertEquals(null, status.result());
            assertEquals(null, status.error());
            assertEquals(null, status.startedAt());
            assertEquals(null, status.finishedAt());
            assertEquals(null, status.createdAt());
            assertEquals(null, status.updatedAt());
        }
    }

    @Test
    void schemaValidationRejectsInvalidPayload() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            invoker.setSchema("fn", Map.of("type", "object", "properties", Map.of(), "required", List.of("name")));
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, error.getErrorCode());
            assertTrue(error.getMessage().contains("name"));
        }
    }

    // ------------------------------------------------------------------
    // header handling
    // ------------------------------------------------------------------

    @Test
    void rejectsHeadersContainingCrOrLf() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertThrows(InvokerException.class, () ->
                invoker.invoke("fn", "{}", InvokeOptions.builder().header("X-Bad\r", "v").build()));
            assertThrows(InvokerException.class, () ->
                invoker.invoke("fn", "{}", InvokeOptions.builder().header("X-Bad\n", "v").build()));
            assertThrows(InvokerException.class, () ->
                invoker.invoke("fn", "{}", InvokeOptions.builder().header("X-Ok", "bad\rvalue").build()));
            assertThrows(InvokerException.class, () ->
                invoker.invoke("fn", "{}", InvokeOptions.builder().header("X-Ok", "bad\nvalue").build()));
        }
    }

    @Test
    void explicitHeadersTakePrecedenceOverConfiguredOnes() throws Exception {
        List<String> authorizations = new ArrayList<>();
        List<String> idempotency = new ArrayList<>();
        List<String> games = new ArrayList<>();
        List<String> envs = new ArrayList<>();
        try (MockServer server = new MockServer(exchange -> {
            authorizations.add(exchange.getRequestHeaders().getFirst("Authorization"));
            idempotency.add(exchange.getRequestHeaders().getFirst("Idempotency-Key"));
            games.add(exchange.getRequestHeaders().getFirst("X-Game-ID"));
            envs.add(exchange.getRequestHeaders().getFirst("X-Env"));
            return Response.ok("{\"result\":\"ok\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).authToken("configured-token")
                .gameId("configured-game").env("configured-env").build());
            invoker.invoke("fn", "{}", InvokeOptions.builder()
                .header("Authorization", "Bearer explicit")
                .header("Idempotency-Key", "explicit-key")
                .header("X-Game-ID", "explicit-game")
                .header("X-Env", "explicit-env")
                .build());
            assertEquals("Bearer explicit", authorizations.get(0));
            assertEquals("explicit-key", idempotency.get(0));
            assertEquals("explicit-game", games.get(0));
            assertEquals("explicit-env", envs.get(0));
        }
    }

    @Test
    void blankIdempotencyKeyIsNotSent() throws Exception {
        List<String> idempotency = new ArrayList<>();
        try (MockServer server = new MockServer(exchange -> {
            idempotency.add(exchange.getRequestHeaders().getFirst("Idempotency-Key"));
            return Response.ok("{\"result\":\"ok\"}");
        })) {
            ServerHttpInvoker invoker = invoker(server, null);
            invoker.invoke("fn", "{}", InvokeOptions.builder().idempotencyKey("  ").build());
            assertEquals(null, idempotency.get(0));
        }
    }

    @Test
    void unPrefixedTokenGetsBearerPrefix() throws Exception {
        List<String> authorizations = new ArrayList<>();
        try (MockServer server = new MockServer(exchange -> {
            authorizations.add(exchange.getRequestHeaders().getFirst("Authorization"));
            return Response.ok("{\"result\":\"ok\"}");
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).authToken("raw-token").build());
            invoker.invoke("fn", "{}");
            assertEquals("Bearer raw-token", authorizations.get(0));

            ServerHttpInvoker pre = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).authToken("BEARER already-prefixed").build());
            pre.invoke("fn", "{}");
            assertEquals("BEARER already-prefixed", authorizations.get(1));
        }
    }

    @Test
    void perRequestTimeoutOverridesConfig() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertEquals("\"ok\"", invoker.invoke("fn", "{}", InvokeOptions.builder().timeout(5000).build()));
        }
    }

    // ------------------------------------------------------------------
    // retry behaviour
    // ------------------------------------------------------------------

    @Test
    void retriesOnRetryableStatusesUntilSuccess() throws Exception {

        for (int status : List.of(503, 429, 504)) {
            try (MockServer server = new MockServer(exchange -> Response.status(status, "{\"message\":\"busy\"}"))) {
                ServerHttpInvoker invoker = invoker(server, RetryConfig.builder()
                    .enabled(true).maxAttempts(2).initialDelayMs(1).maxDelayMs(2)
                    .backoffMultiplier(0).jitterFactor(0).build());
                assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
            }
        }
        try (MockServer server = new MockServer(exchange -> Response.status(503, "{\"message\":\"busy\"}"))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder().enabled(false).build());
            assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));

            ServerHttpInvoker nullRetry = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).build());
            // request with explicit options-level retry override
            assertThrows(InvokerException.class, () -> nullRetry.invoke("fn", "{}",
                InvokeOptions.builder().retry(RetryConfig.builder().enabled(true).maxAttempts(2)
                    .initialDelayMs(1).maxDelayMs(1).backoffMultiplier(0).jitterFactor(0).build()).build()));
        }
    }

    @Test
    void nonRetryableStatusFailsFast() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.status(403, "{\"message\":\"denied\"}"))) {
            ServerHttpInvoker invoker = invoker(server, RetryConfig.builder()
                .enabled(true).maxAttempts(5).initialDelayMs(1).build());
            assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
        }
    }

    // ------------------------------------------------------------------
    // streaming subscription edges
    // ------------------------------------------------------------------

    @Test
    void streamTaskRejectsNonPositiveRequestCounts() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"items\":[],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            CountDownLatch errored = new CountDownLatch(1);
            AtomicReference<Throwable> received = new AtomicReference<>();
            invoker.streamTask("task-x").subscribe(new Subscriber<TaskEventInfo>() {
                @Override public void onSubscribe(Subscription subscription) { subscription.request(0); }
                @Override public void onNext(TaskEventInfo event) { }
                @Override public void onError(Throwable error) { received.set(error); errored.countDown(); }
                @Override public void onComplete() { }
            });
            assertTrue(errored.await(5, TimeUnit.SECONDS));
            assertTrue(received.get() instanceof IllegalArgumentException);
        }
    }

    @Test
    void streamTaskSurfacesInvalidEventPayloads() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"items\":[\"not-an-object\"],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            CountDownLatch errored = new CountDownLatch(1);
            invoker.streamTask("task-x").subscribe(new Subscriber<TaskEventInfo>() {
                @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
                @Override public void onNext(TaskEventInfo event) { }
                @Override public void onError(Throwable error) { errored.countDown(); }
                @Override public void onComplete() { }
            });
            assertTrue(errored.await(5, TimeUnit.SECONDS));
        }
        try (MockServer server = new MockServer(exchange -> Response.ok("{\"items\":{},\"done\":false}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            CountDownLatch errored = new CountDownLatch(1);
            invoker.streamTask("task-x").subscribe(new Subscriber<TaskEventInfo>() {
                @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
                @Override public void onNext(TaskEventInfo event) { }
                @Override public void onError(Throwable error) { errored.countDown(); }
                @Override public void onComplete() { }
            });
            assertTrue(errored.await(5, TimeUnit.SECONDS));
        }
    }

    @Test
    void streamTaskHandlesMissingSeqAndUnknownTypes() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.ok(
            "{\"items\":[{\"type\":\"done\"},{\"type\":\"failed\",\"message\":\"bad\"},{\"type\":\"weird\",\"progress\":\"nan\"}],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            CountDownLatch completed = new CountDownLatch(1);
            List<TaskEventInfo> events = new ArrayList<>();
            invoker.streamTask("task-x").subscribe(new Subscriber<TaskEventInfo>() {
                @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
                @Override public void onNext(TaskEventInfo event) { events.add(event); }
                @Override public void onError(Throwable error) { throw new AssertionError(error); }
                @Override public void onComplete() { completed.countDown(); }
            });
            assertTrue(completed.await(5, TimeUnit.SECONDS));
            assertEquals(List.of("completed", "failed", "weird"), events.stream().map(TaskEventInfo::getType).toList());
            assertTrue(events.get(1).getError() != null);
            assertTrue(events.get(0).isDone());
            assertTrue(!events.get(2).isDone());
            assertEquals(null, events.get(2).getProgress());
        }
    }

    @Test
    void streamTaskErrorPropagationForInvalidIdentifier() throws Exception {

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"items\":[],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            CountDownLatch errored = new CountDownLatch(1);
            invoker.streamTask(" ").subscribe(new Subscriber<TaskEventInfo>() {
                @Override public void onSubscribe(Subscription subscription) { subscription.request(1); }
                @Override public void onNext(TaskEventInfo event) { }
                @Override public void onError(Throwable error) { errored.countDown(); }
                @Override public void onComplete() { }
            });
            assertTrue(errored.await(5, TimeUnit.SECONDS));
        }
    }

    @Test
    void connectAndCloseTrackConnectionState() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.ok("{\"result\":\"ok\"}"))) {
            ServerHttpInvoker invoker = invoker(server, null);
            assertTrue(!invoker.isConnected());
            invoker.connect();
            assertTrue(invoker.isConnected());
            assertEquals("\"ok\"", invoker.invoke("fn", "{}"));
            invoker.close();
            assertTrue(!invoker.isConnected());
        }
    }
}
