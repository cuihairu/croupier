package io.github.cuihairu.croupier.sdk.invoker;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Interrupt-path coverage:
 * <ul>
 *   <li>retry backoff interrupted -> {@code InvokerException(CANCELLED, "Server HTTP request interrupted")}</li>
 *   <li>stream-task poll worker interrupted -> subscription ends silently (no onError/onComplete)</li>
 * </ul>
 */
class ServerHttpInvokerInterruptTest {

    private static int deadPort() throws IOException {
        try (ServerSocket socket = new ServerSocket()) {
            socket.bind(new InetSocketAddress("127.0.0.1", 0));
            socket.setReuseAddress(false);
            return socket.getLocalPort();
        }
    }

    @Test
    void invokeInterruptedDuringRetryBackoffThrowsCancelled() throws Exception {
        RetryConfig retry = RetryConfig.builder()
                .enabled(true)
                .maxAttempts(3)
                .initialDelayMs(10_000)
                .maxDelayMs(10_000)
                .build();
        ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                .address("http://127.0.0.1:" + deadPort())
                .taskPollIntervalMs(1)
                .retry(retry)
                .build());

        AtomicReference<Throwable> captured = new AtomicReference<>();
        CountDownLatch finished = new CountDownLatch(1);
        Thread worker = new Thread(() -> {
            try {
                invoker.invoke("fn.echo", "{}", null);
            } catch (Throwable error) {
                captured.set(error);
            } finally {
                finished.countDown();
            }
        }, "invoke-interrupt-test");
        worker.start();

        // The refused connection fails in milliseconds; the 10s backoff sleep
        // dominates, so interrupting here lands inside Thread.sleep reliably.
        Thread.sleep(800);
        worker.interrupt();
        assertTrue(finished.await(5, TimeUnit.SECONDS), "invoke must finish after interrupt");

        Throwable thrown = captured.get();
        assertNotNull(thrown, "interrupted invoke must throw");
        assertEquals(InvokerException.class, thrown.getClass());
        InvokerException exception = (InvokerException) thrown;
        assertEquals(InvokerException.ErrorCode.CANCELLED, exception.getErrorCode());
        assertTrue(exception.getMessage().contains("interrupted"), () -> "unexpected message: " + exception.getMessage());
        invoker.close();
    }

    /** Server whose /events endpoint instantly replies with empty items and done=false forever. */
    private static final class NeverDoneEventsServer implements AutoCloseable {
        private final HttpServer server;
        final java.util.concurrent.atomic.AtomicInteger requests = new java.util.concurrent.atomic.AtomicInteger();

        NeverDoneEventsServer() throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", this::respond);
            server.start();
        }

        private void respond(HttpExchange exchange) throws IOException {
            requests.incrementAndGet();
            byte[] body = "{\"items\":[],\"done\":false}".getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().add("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, body.length);
            exchange.getResponseBody().write(body);
            exchange.close();
        }

        String baseUrl() {
            return "http://127.0.0.1:" + server.getAddress().getPort();
        }

        @Override
        public void close() {
            server.stop(0);
        }
    }

    private static final class RecordingSubscriber implements Subscriber<TaskEventInfo> {
        final CountDownLatch subscribed = new CountDownLatch(1);
        final AtomicReference<Subscription> subscription = new AtomicReference<>();
        final AtomicReference<Throwable> error = new AtomicReference<>();
        volatile boolean completed;

        @Override
        public void onSubscribe(Subscription s) {
            subscription.set(s);
            subscribed.countDown();
        }

        @Override
        public void onNext(TaskEventInfo event) {
            // no-op: server never emits items
        }

        @Override
        public void onError(Throwable t) {
            error.set(t);
        }

        @Override
        public void onComplete() {
            completed = true;
        }
    }

    @Test
    void streamTaskWorkerInterruptedDuringPollEndsSilently() throws Exception {
        try (NeverDoneEventsServer server = new NeverDoneEventsServer()) {
            ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
                    .address(server.baseUrl())
                    .taskPollIntervalMs(300)
                    .retry(RetryConfig.builder().enabled(false).build())
                    .build());

            RecordingSubscriber subscriber = new RecordingSubscriber();
            invoker.streamTask("task-1").subscribe(subscriber);
            assertTrue(subscriber.subscribed.await(5, TimeUnit.SECONDS), "onSubscribe must fire");
            subscriber.subscription.get().request(1);

            // Wait until the first poll response has been consumed, then a short
            // beat so the worker sits inside Thread.sleep(pollInterval): the
            // interrupt then deterministically hits the poll sleep instead of
            // the HTTP round-trip. If the race is lost, the interrupt surfaces
            // as onError(CANCELLED) via the generic catch — also acceptable.
            for (int i = 0; i < 150 && server.requests.get() < 1; i++) {
                Thread.sleep(20);
            }
            assertTrue(server.requests.get() >= 1, "first poll must reach the server");
            Thread worker = waitForWorkerThread("croupier-server-task-events-task-1");
            assertNotNull(worker, "stream worker thread must be running");
            Thread.sleep(80);
            worker.interrupt();
            worker.join(TimeUnit.SECONDS.toMillis(5));
            assertFalse(worker.isAlive(), "interrupted worker must exit");

            // Give any terminal signal a moment to surface.
            Thread.sleep(300);
            assertFalse(subscriber.completed, "interrupt must not surface onComplete");
            Throwable surfaced = subscriber.error.get();
            if (surfaced != null) {
                assertEquals(InvokerException.class, surfaced.getClass(),
                        () -> "unexpected terminal error: " + surfaced);
                assertEquals(InvokerException.ErrorCode.CANCELLED,
                        ((InvokerException) surfaced).getErrorCode(),
                        () -> "unexpected terminal error: " + surfaced);
            }

            subscriber.subscription.get().cancel();
            invoker.close();
        }
    }

    private static Thread waitForWorkerThread(String namePrefix) throws InterruptedException {
        for (int i = 0; i < 150; i++) {
            for (Thread thread : Thread.getAllStackTraces().keySet()) {
                if (thread.getName().startsWith(namePrefix)) {
                    return thread;
                }
            }
            Thread.sleep(20);
        }
        return null;
    }
}
