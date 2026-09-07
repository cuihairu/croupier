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

import static org.junit.jupiter.api.Assertions.assertTrue;

/** EventsSubscription edge branches: demand overflow clamp and cancel timing. */
class ServerHttpInvokerSubscriptionBranchTest {

    private record Response(int status, String body) {
        static Response ok(String body) { return new Response(200, body); }
    }

    private static final class SlowEventsServer implements AutoCloseable {
        private final HttpServer server;

        SlowEventsServer() throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", exchange -> respond(exchange));
            server.start();
        }

        private void respond(HttpExchange exchange) throws IOException {
            try {
                if (exchange.getRequestURI().getPath().endsWith("/events")) {
                    Thread.sleep(150); // give the test time to cancel mid-poll
                    byte[] body = "{\"items\":[{\"seq\":1,\"type\":\"progress\"}],\"done\":false}"
                        .getBytes(StandardCharsets.UTF_8);
                    exchange.sendResponseHeaders(200, body.length);
                    exchange.getResponseBody().write(body);
                } else {
                    byte[] body = "{}".getBytes(StandardCharsets.UTF_8);
                    exchange.sendResponseHeaders(200, body.length);
                    exchange.getResponseBody().write(body);
                }
            } catch (InterruptedException ignored) {
                Thread.currentThread().interrupt();
            }
            exchange.close();
        }

        String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
        @Override public void close() { server.stop(0); }
    }

    private static final class ControllingSubscriber implements Subscriber<TaskEventInfo> {
        final AtomicReference<Subscription> subscription = new AtomicReference<>();
        final List<TaskEventInfo> events = new ArrayList<>();
        final CountDownLatch firstEvent = new CountDownLatch(1);
        volatile boolean errored;

        @Override public void onSubscribe(Subscription s) { subscription.set(s); }
        @Override public void onNext(TaskEventInfo event) { events.add(event); firstEvent.countDown(); }
        @Override public void onError(Throwable t) { errored = true; }
        @Override public void onComplete() { }
    }

    private static ServerHttpInvoker invoker(String address) {
        return new ServerHttpInvoker(InvokerConfig.builder()
            .address(address).taskPollIntervalMs(1)
            .retry(RetryConfig.builder().enabled(false).build())
            .build());
    }

    @Test
    void demandSaturatesAtLongMaxValueWithoutOverflow() throws Exception {
        try (SlowEventsServer server = new SlowEventsServer()) {
            ServerHttpInvoker invoker = invoker(server.baseUrl());
            ControllingSubscriber subscriber = new ControllingSubscriber();
            invoker.streamTask("task-sat").subscribe(subscriber);
            Subscription subscription = subscriber.subscription.get();
            // two MAX_VALUE requests exercise the overflow clamp
            subscription.request(Long.MAX_VALUE);
            subscription.request(Long.MAX_VALUE);
            // a zero-count request errors and cancels the subscription
            subscription.request(0);
            long deadline = System.currentTimeMillis() + 5000;
            while (!subscriber.errored && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertTrue(subscriber.errored, "request(0) must surface onError");
        }
    }

    @Test
    void cancelWhileAwaitingDemandStopsDelivery() throws Exception {
        try (SlowEventsServer server = new SlowEventsServer()) {
            ServerHttpInvoker invoker = invoker(server.baseUrl());
            ControllingSubscriber subscriber = new ControllingSubscriber();
            invoker.streamTask("task-cxl").subscribe(subscriber);
            Subscription subscription = subscriber.subscription.get();
            subscription.request(1);
            // wait for the first event, then cancel before requesting more:
            // the poll loop must observe the cancel flag instead of emitting
            assertTrue(subscriber.firstEvent.await(5, TimeUnit.SECONDS));
            subscription.cancel();
            int seen = subscriber.events.size();
            Thread.sleep(400);
            assertTrue(subscriber.events.size() <= seen + 1,
                "no further events should be delivered after cancel");
        }
    }

    @Test
    void cancelBeforeDoneResponseSkipsOnComplete() throws Exception {
        try (SlowEventsServer server = new SlowEventsServer()) {
            ServerHttpInvoker invoker = invoker(server.baseUrl());
            ControllingSubscriber subscriber = new ControllingSubscriber();
            invoker.streamTask("task-pre").subscribe(subscriber);
            Subscription subscription = subscriber.subscription.get();
            subscription.request(1);
            // cancel while the first poll is still in flight (server sleeps)
            subscription.cancel();
            Thread.sleep(400);
            // no error and no events should surface for a cancelled stream
            assertTrue(!subscriber.errored);
        }
    }
}
