package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Second-wave branch fillers for InvokerImpl event typing edges. */
class InvokerImplEventBranchTest {

    private static InvokerConfig config() {
        return InvokerConfig.builder()
            .address("127.0.0.1:19090").insecure(true).timeout(2000)
            .retry(RetryConfig.builder().enabled(false).build())
            .build();
    }

    private static final class TypedEvents implements AutoCloseable {
        final FakeTransportClient transport;
        final InvokerImpl invoker;
        final CountDownLatch received = new CountDownLatch(2);
        final List<TaskEventInfo> events = new java.util.ArrayList<>();

        TypedEvents(String taskId, SdkWireMessages.TaskEvent polled) {
            transport = new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                    return SdkWireMessages.encodeStartTaskResponse(
                        new SdkWireMessages.StartTaskResponse(taskId));
                }
                return SdkWireMessages.encodeTaskEvent(polled);
            });
            invoker = new InvokerImpl(config(), (address, timeout) -> transport);
            assertDoesNotThrow(invoker::connect);
        }

        String start() {
            return assertDoesNotThrow(() -> invoker.startTask("fn", "{}"));
        }

        void subscribe() {
            invoker.streamTask(start()).subscribe(new Subscriber<TaskEventInfo>() {
                @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
                @Override public void onNext(TaskEventInfo event) { events.add(event); received.countDown(); }
                @Override public void onError(Throwable error) { }
                @Override public void onComplete() { }
            });
        }

        @Override public void close() { assertDoesNotThrow(invoker::close); }
    }

    @Test
    void errorEventWithoutCancelMessageKeepsErrorTypeAndMessage() throws Exception {
        try (TypedEvents harness = new TypedEvents("task-err",
            new SdkWireMessages.TaskEvent("error", "disk full", 100, new byte[0]))) {
            harness.subscribe();
            assertTrue(harness.received.await(5, TimeUnit.SECONDS));
            assertEquals("started", harness.events.get(0).getType());
            assertEquals("error", harness.events.get(1).getType());
            assertTrue(harness.events.get(1).isDone());
            assertEquals("disk full", harness.events.get(1).getError());
        }
    }

    @Test
    void errorEventWithBlankMessageCoversNullMessageBranch() throws Exception {
        try (TypedEvents harness = new TypedEvents("task-blank",
            new SdkWireMessages.TaskEvent("error", "", 100, new byte[0]))) {
            harness.subscribe();
            assertTrue(harness.received.await(5, TimeUnit.SECONDS));
            assertEquals("error", harness.events.get(1).getType());
            assertEquals("", harness.events.get(1).getError());
        }
    }

    @Test
    void doneTypeMapsToCompletedWithPayloadAndErrorNull() throws Exception {
        try (TypedEvents harness = new TypedEvents("task-done",
            new SdkWireMessages.TaskEvent("done", "finished", 100,
                "{\"ok\":1}".getBytes(StandardCharsets.UTF_8)))) {
            harness.subscribe();
            assertTrue(harness.received.await(5, TimeUnit.SECONDS));
            assertEquals("completed", harness.events.get(1).getType());
            assertTrue(harness.events.get(1).isDone());
            assertEquals(null, harness.events.get(1).getError());
        }
    }

    @Test
    void simulateProgressAfterCancelIsIgnoredForDoneState() throws Exception {
        try (TypedEvents harness = new TypedEvents("task-cancelled",
            new SdkWireMessages.TaskEvent("progress", "working", 10, new byte[0]))) {
            String taskId = harness.start();
            assertDoesNotThrow(() -> harness.invoker.cancelTask(taskId));
            // task is now done: simulateTaskProgress must skip publishing
            assertDoesNotThrow(() -> harness.invoker.simulateTaskProgress(taskId, 99, "late"));
        }
    }
}
