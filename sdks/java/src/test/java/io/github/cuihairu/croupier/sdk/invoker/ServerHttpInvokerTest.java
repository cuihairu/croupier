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

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Mock Server HTTP contract tests for the public Java L3 Invoker. */
class ServerHttpInvokerTest {

    @Test
    void usesServerHttpContractForInvokeAndTaskLifecycle() throws Exception {
        List<RecordedRequest> requests = new ArrayList<>();
        try (MockServer server = new MockServer(exchange -> {
            requests.add(RecordedRequest.from(exchange));
            return switch (exchange.getRequestURI().getPath()) {
                case "/api/v1/functions/player.ban/invoke" -> Response.ok("{\"result\":{\"status\":\"banned\"}}");
                case "/api/v1/tasks" -> Response.ok("{\"taskId\":\"task-1\",\"status\":\"dispatching\"}");
                case "/api/v1/tasks/task-1" -> Response.ok("{\"id\":\"task-1\",\"functionId\":\"report.generate\",\"status\":\"running\",\"progress\":50,\"result\":{\"partial\":true}}");
                case "/api/v1/tasks/task-1/events" -> exchange.getRequestURI().getQuery().equals("after_seq=0")
                    ? Response.ok("{\"items\":[{\"seq\":1,\"type\":\"progress\",\"progress\":50,\"payload\":{\"count\":1}}],\"done\":false}")
                    : Response.ok("{\"items\":[{\"seq\":2,\"type\":\"completed\",\"payload\":{\"ok\":true}}],\"done\":true}");
                case "/api/v1/tasks/task-1/cancel" -> Response.ok("{\"message\":\"accepted\"}");
                default -> Response.status(404, "{\"message\":\"unexpected path\"}");
            };
        })) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).authToken("server-token").gameId("game-a").env("staging")
                .taskPollIntervalMs(1).retry(RetryConfig.builder().enabled(false).build()).build());

            String result = invoker.invoke("player.ban", "{\"playerId\":\"p-1\"}", InvokeOptions.builder().idempotencyKey("invoke-1").build());
            String taskId = invoker.startTask("report.generate", "{\"range\":\"daily\"}");
            TaskStatusInfo status = invoker.getTaskStatus(taskId);
            CollectingSubscriber subscriber = new CollectingSubscriber();
            invoker.streamTask(taskId).subscribe(subscriber);
            assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS));
            invoker.cancelTask(taskId);

            assertEquals("{\"status\":\"banned\"}", result);
            assertEquals("task-1", taskId);
            assertEquals("running", status.status());
            assertEquals("{\"partial\":true}", status.result());
            assertEquals(List.of("progress", "completed"), subscriber.events.stream().map(TaskEventInfo::getType).toList());
        }

        assertEquals("POST", requests.get(0).method());
        assertEquals("/api/v1/functions/player.ban/invoke", requests.get(0).path());
        assertEquals("Bearer server-token", requests.get(0).header("Authorization"));
        assertEquals("game-a", requests.get(0).header("X-Game-ID"));
        assertEquals("staging", requests.get(0).header("X-Env"));
        assertEquals("invoke-1", requests.get(0).header("Idempotency-Key"));
        assertEquals("{\"params\":{\"playerId\":\"p-1\"}}", requests.get(0).body());
        assertEquals(List.of(
            "/api/v1/functions/player.ban/invoke", "/api/v1/tasks", "/api/v1/tasks/task-1",
            "/api/v1/tasks/task-1/events", "/api/v1/tasks/task-1/events", "/api/v1/tasks/task-1/cancel"
        ), requests.stream().map(RecordedRequest::path).toList());
    }

    @Test
    void rejectsMissingTaskIdInvalidInputAndServerErrors() throws Exception {
        try (MockServer server = new MockServer(exchange -> Response.status(403, "{\"message\":\"scope denied\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address(server.baseUrl()).retry(RetryConfig.builder().enabled(false).build()).build());
            assertThrows(InvokerException.class, () -> invoker.invoke("player.ban", "{}"));
            assertThrows(InvokerException.class, () -> invoker.invoke("", "{}"));
            assertThrows(InvokerException.class, () -> invoker.startTask("report.generate", "not-json"));
            assertThrows(InvokerException.class, () -> invoker.getTaskStatus(" "));
        }

        try (MockServer server = new MockServer(exchange -> Response.ok("{\"status\":\"dispatching\"}"))) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder().address(server.baseUrl()).build());
            InvokerException error = assertThrows(InvokerException.class, () -> invoker.startTask("report.generate", "{}"));
            assertTrue(error.getMessage().contains("taskId"));
        }
    }

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

    private record RecordedRequest(String method, String path, Map<String, List<String>> headers, String body) {
        static RecordedRequest from(HttpExchange exchange) throws IOException {
            return new RecordedRequest(exchange.getRequestMethod(), exchange.getRequestURI().getPath(), exchange.getRequestHeaders(), new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
        }
        String header(String name) { return headers.entrySet().stream().filter(entry -> entry.getKey().equalsIgnoreCase(name)).findFirst().map(entry -> entry.getValue().get(0)).orElse(null); }
    }

    private static final class CollectingSubscriber implements Subscriber<TaskEventInfo> {
        private final List<TaskEventInfo> events = new ArrayList<>();
        private final CountDownLatch completed = new CountDownLatch(1);
        @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
        @Override public void onNext(TaskEventInfo event) { events.add(event); }
        @Override public void onError(Throwable error) { throw new AssertionError(error); }
        @Override public void onComplete() { completed.countDown(); }
    }
}
