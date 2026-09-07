package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertTrue;

/** Deterministic cancel/error races for EventsSubscription branch coverage. */
class ServerHttpInvokerRaceBranchTest {

    /**
     * Server whose /events handler sleeps before answering, so the test can
     * cancel while the poll is deterministically in flight.
     */
    private static final class SlowServer implements AutoCloseable {
        private final HttpServer server;
        private volatile String body = "{\"items\":[{\"seq\":1,\"type\":\"progress\"}],\"done\":false}";
        private volatile long delayMs = 800;

        SlowServer() throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> {
                try {
                    if (exchange.getRequestURI().getPath().endsWith("/events")) {
                        Thread.sleep(delayMs);
                    }
                    byte[] payload = body.getBytes(StandardCharsets.UTF_8);
                    exchange.sendResponseHeaders(200, payload.length);
                    exchange.getResponseBody().write(payload);
                } catch (InterruptedException ignored) {
                    Thread.currentThread().interrupt();
                }
                exchange.close();
            });
            server.start();
        }

        void respondAfter(long delayMs, String body) {
            this.delayMs = delayMs;
            this.body = body;
        }
        String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
        @Override public void close() { server.stop(0); }
    }

    private static ServerHttpInvoker invoker(String address) {
        return new ServerHttpInvoker(InvokerConfig.builder()
            .address(address).taskPollIntervalMs(1)
            .retry(RetryConfig.builder().enabled(false).build())
            .build());
    }

    private static final class Tracking implements Subscriber<TaskEventInfo> {
        final AtomicReference<Subscription> subscription = new AtomicReference<>();
        final AtomicBoolean errored = new AtomicBoolean();
        final AtomicBoolean completed = new AtomicBoolean();
        final CountDownLatch subscribed = new CountDownLatch(1);

        @Override public void onSubscribe(Subscription s) { subscription.set(s); subscribed.countDown(); }
        @Override public void onNext(TaskEventInfo event) { }
        @Override public void onError(Throwable t) { errored.set(true); }
        @Override public void onComplete() { completed.set(true); }
    }

    @Test
    void doneResponseAfterCancelSkipsOnComplete() throws Exception {
        try (SlowServer server = new SlowServer()) {
            server.respondAfter(800, "{\"items\":[],\"done\":true}");
            ServerHttpInvoker invoker = invoker(server.baseUrl());
            Tracking subscriber = new Tracking();
            invoker.streamTask("task-a").subscribe(subscriber);
            assertTrue(subscriber.subscribed.await(5, TimeUnit.SECONDS));
            subscriber.subscription.get().request(1);
            // cancel while the done poll is blocked on the server
            Thread.sleep(200);
            subscriber.subscription.get().cancel();
            Thread.sleep(1200);
            assertTrue(!subscriber.completed.get(), "onComplete must be skipped when cancelled");
            assertTrue(!subscriber.errored.get(), "no error expected");
        }
    }

    @Test
    void decodeErrorAfterCancelSkipsOnError() throws Exception {
        try (SlowServer server = new SlowServer()) {
            ServerHttpInvoker invoker = invoker(server.baseUrl());
            Tracking subscriber = new Tracking();
            invoker.streamTask("task-b").subscribe(subscriber);
            assertTrue(subscriber.subscribed.await(5, TimeUnit.SECONDS));
            subscriber.subscription.get().request(Long.MAX_VALUE);
            // first poll succeeds, then poison the stream and cancel mid-poll
            server.respondAfter(600, "{not-json");
            Thread.sleep(150);
            subscriber.subscription.get().cancel();
            Thread.sleep(1500);
            assertTrue(!subscriber.errored.get(), "onError must be skipped when cancelled");
        }
    }

    @Test
    void cancelWhileAwaitingDemandExitsWorkerLoop() throws Exception {
        try (SlowServer server = new SlowServer()) {
            // two events in one response: the first consumes the single demand,
            // the second parks in awaitDemand until cancel releases it
            server.respondAfter(200, "{\"items\":["
                + "{\"seq\":1,\"type\":\"progress\"},{\"seq\":2,\"type\":\"progress\"}],\"done\":false}");
            ServerHttpInvoker invoker = invoker(server.baseUrl());
            Tracking subscriber = new Tracking();
            invoker.streamTask("task-c").subscribe(subscriber);
            assertTrue(subscriber.subscribed.await(5, TimeUnit.SECONDS));
            subscriber.subscription.get().request(1);
            Thread.sleep(1000); // both events fetched; worker parks awaiting demand
            subscriber.subscription.get().cancel();
            Thread.sleep(500);
            assertTrue(!subscriber.errored.get(), "cancel must not surface an error");
        }
    }
}
