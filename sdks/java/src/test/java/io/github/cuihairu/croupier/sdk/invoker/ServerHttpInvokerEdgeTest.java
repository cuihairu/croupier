package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Function;

import static org.junit.jupiter.api.Assertions.*;

/**
 * ServerHttpInvoker 边缘补测：重试中断、请求中断、查询串拼接、
 * retryDelay/serverErrorMessage/normalizeBaseUrl 私有工具、
 * EventsSubscription 的 request(0)、逐条 demand 与轮询错误路径。
 */
@DisplayName("ServerHttpInvoker retry/interrupt/endpoint edge paths")
class ServerHttpInvokerEdgeTest {

    private record Response(int status, String body) {
        static Response ok(String body) { return new Response(200, body); }
        static Response status(int status, String body) { return new Response(status, body); }
    }

    private static final class MockServer implements AutoCloseable {
        final HttpServer server;
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

    private static RetryConfig noRetry() {
        return RetryConfig.builder().enabled(false).build();
    }

    @Test
    @DisplayName("retry sleep is interruptible and surfaces CANCELLED")
    void retrySleepInterrupt() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        try (MockServer server = new MockServer(ex -> {
            calls.incrementAndGet();
            return Response.status(503, "{\"message\":\"down\"}");
        })) {
            RetryConfig retry = RetryConfig.builder()
                .maxAttempts(10).initialDelayMs(10_000).maxDelayMs(10_000).build();
            ServerHttpInvoker invoker = invoker(server.baseUrl(), retry);
            AtomicReference<Throwable> failure = new AtomicReference<>();
            CountDownLatch done = new CountDownLatch(1);
            Thread caller = new Thread(() -> {
                try {
                    invoker.invoke("fn", "{}");
                } catch (Throwable t) {
                    failure.set(t);
                } finally {
                    done.countDown();
                }
            });
            caller.start();
            // 等第一次请求落地，此时主线程进入重试 sleep
            awaitValue(calls, 1, 5000);
            caller.interrupt();
            assertTrue(done.await(5, TimeUnit.SECONDS));
            assertNotNull(failure.get());
            assertTrue(failure.get() instanceof InvokerException);
            assertEquals(InvokerException.ErrorCode.CANCELLED,
                ((InvokerException) failure.get()).getErrorCode());
            invoker.close();
        }
    }

    @Test
    @DisplayName("in-flight HTTP send interrupted surfaces CANCELLED")
    void requestSendInterrupt() throws Exception {
        CountDownLatch requestArrived = new CountDownLatch(1);
        try (MockServer server = new MockServer(ex -> {
            requestArrived.countDown();
            try {
                Thread.sleep(3000);
            } catch (InterruptedException ignored) {
                Thread.currentThread().interrupt();
            }
            return Response.ok("{}");
        })) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            AtomicReference<Throwable> failure = new AtomicReference<>();
            CountDownLatch done = new CountDownLatch(1);
            Thread caller = new Thread(() -> {
                try {
                    invoker.invoke("fn", "{}");
                } catch (Throwable t) {
                    failure.set(t);
                } finally {
                    done.countDown();
                }
            });
            caller.start();
            assertTrue(requestArrived.await(5, TimeUnit.SECONDS));
            caller.interrupt();
            assertTrue(done.await(5, TimeUnit.SECONDS));
            assertNotNull(failure.get());
            assertEquals(InvokerException.ErrorCode.CANCELLED,
                ((InvokerException) failure.get()).getErrorCode());
            invoker.close();
        }
    }

    @Test
    @DisplayName("endpoint joins multiple query parameters with &")
    void endpointJoinsQueryParameters() throws Exception {
        ServerHttpInvoker invoker = invoker("http://127.0.0.1:1", null);
        java.lang.reflect.Method endpoint = ServerHttpInvoker.class
            .getDeclaredMethod("endpoint", List.class, Map.class);
        endpoint.setAccessible(true);
        URI uri = (URI) endpoint.invoke(invoker, List.of("tasks", "t 1"),
            Map.of("after_seq", "3", "lang", "zh CN"));
        assertTrue(uri.toString().contains("&"));
        assertTrue(uri.toString().contains("after_seq=3"));
        assertTrue(uri.toString().contains("lang=zh%20CN"));
        invoker.close();
    }

    @Test
    @DisplayName("retryDelay respects max delay and jitter bounds")
    void retryDelayBounds() throws Exception {
        ServerHttpInvoker invoker = invoker("http://127.0.0.1:1", null);
        java.lang.reflect.Method retryDelay = ServerHttpInvoker.class
            .getDeclaredMethod("retryDelay", int.class, RetryConfig.class);
        retryDelay.setAccessible(true);
        RetryConfig retry = RetryConfig.builder()
            .initialDelayMs(500).maxDelayMs(700).jitterFactor(0.5).build();
        for (int attempt = 0; attempt < 3; attempt++) {
            long delay = (Long) retryDelay.invoke(null, attempt, retry);
            assertTrue(delay >= 0 && delay <= 1400, "delay out of bounds: " + delay);
        }
        invoker.close();
    }

    @Test
    @DisplayName("non-JSON error body is surfaced verbatim")
    void nonJsonErrorBodySurfaced() throws Exception {
        try (MockServer server = new MockServer(ex -> Response.status(500, "not-json"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), noRetry());
            InvokerException error = assertThrows(InvokerException.class,
                () -> invoker.invoke("fn", "{}"));
            assertTrue(error.getMessage().contains("not-json"));
            invoker.close();
        }
    }

    @Test
    @DisplayName("normalizeBaseUrl accepts hosts, strips trailing slashes, rejects non-HTTP")
    void normalizeBaseUrlVariants() throws Exception {
        java.lang.reflect.Method normalize = ServerHttpInvoker.class
            .getDeclaredMethod("normalizeBaseUrl", String.class);
        normalize.setAccessible(true);
        assertEquals("http://127.0.0.1:1234/api/v1", normalize.invoke(null, "127.0.0.1:1234"));
        assertEquals("https://host/api/v1", normalize.invoke(null, "https://host///"));
        assertEquals("http://host/base/api/v1", normalize.invoke(null, "http://host/base/"));
        assertEquals("http://host/api/v1", normalize.invoke(null, "http://host/api/v1/"));
        try {
            Object rejected = normalize.invoke(null, "ftp://host");
            fail("expected rejection but got " + rejected);
        } catch (java.lang.reflect.InvocationTargetException e) {
            assertTrue(e.getCause() instanceof IllegalArgumentException);
        }
    }

    @Test
    @DisplayName("subscription request(0) cancels with error")
    void streamTaskNonPositiveDemandRejected() throws Exception {
        try (MockServer server = new MockServer(ex -> Response.ok("{\"items\":[],\"done\":false}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            CountDownLatch errored = new CountDownLatch(1);
            AtomicReference<Throwable> seen = new AtomicReference<>();
            invoker.streamTask("t-1").subscribe(new Subscriber<>() {
                @Override public void onSubscribe(Subscription s) { s.request(0); }
                @Override public void onNext(TaskEventInfo event) { }
                @Override public void onError(Throwable t) { seen.set(t); errored.countDown(); }
                @Override public void onComplete() { }
            });
            assertTrue(errored.await(5, TimeUnit.SECONDS));
            assertTrue(seen.get() instanceof IllegalArgumentException);
            invoker.close();
        }
    }

    @Test
    @DisplayName("events are emitted respecting demand; poller errors surface onError")
    void streamTaskDemandControlledAndPollFailure() throws Exception {
        // 一次响应两条事件 + done：第二条事件需等待补充 demand（覆盖 awaitDemand park）
        try (MockServer server = new MockServer(ex -> Response.ok(
            "{\"items\":["
                + "{\"seq\":1,\"type\":\"progress\",\"message\":\"first\"},"
                + "{\"seq\":2,\"type\":\"completed\",\"message\":\"second\"}"
                + "],\"done\":true}"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), null);
            List<TaskEventInfo> events = new CopyOnWriteArrayList<>();
            CountDownLatch firstSeen = new CountDownLatch(1);
            CountDownLatch completed = new CountDownLatch(1);
            AtomicReference<Subscription> subscriptionRef = new AtomicReference<>();
            invoker.streamTask("t-2").subscribe(new Subscriber<>() {
                @Override public void onSubscribe(Subscription s) { subscriptionRef.set(s); s.request(1); }
                @Override public void onNext(TaskEventInfo event) {
                    events.add(event);
                    if (events.size() == 1) {
                        firstSeen.countDown();
                    }
                }
                @Override public void onError(Throwable t) { completed.countDown(); }
                @Override public void onComplete() { completed.countDown(); }
            });
            assertTrue(firstSeen.await(5, TimeUnit.SECONDS));
            // 让 worker 在 awaitDemand 停留片刻后再补充 demand
            Thread.sleep(200);
            subscriptionRef.get().request(1);
            assertTrue(completed.await(5, TimeUnit.SECONDS));
            assertEquals(2, events.size());
            invoker.close();
        }

        // 轮询期间服务端 500：worker 捕获并 onError
        try (MockServer server = new MockServer(ex -> Response.status(500, "boom"))) {
            ServerHttpInvoker invoker = invoker(server.baseUrl(), noRetry());
            CountDownLatch errored = new CountDownLatch(1);
            AtomicReference<Throwable> seen = new AtomicReference<>();
            invoker.streamTask("t-3").subscribe(new Subscriber<>() {
                @Override public void onSubscribe(Subscription s) { s.request(Long.MAX_VALUE); }
                @Override public void onNext(TaskEventInfo event) { }
                @Override public void onError(Throwable t) { seen.set(t); errored.countDown(); }
                @Override public void onComplete() { }
            });
            assertTrue(errored.await(10, TimeUnit.SECONDS));
            assertTrue(seen.get() instanceof InvokerException);
            invoker.close();
        }
    }

    private static void awaitValue(AtomicInteger counter, int expected, long timeoutMs)
            throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (counter.get() < expected && System.currentTimeMillis() < deadline) {
            Thread.sleep(10);
        }
    }
}
