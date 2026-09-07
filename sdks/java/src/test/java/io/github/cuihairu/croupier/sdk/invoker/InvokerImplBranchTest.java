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
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for InvokerImpl retry/polling/normalization edges. */
class InvokerImplBranchTest {

    private static InvokerConfig config(RetryConfig retry) {
        return InvokerConfig.builder()
            .address("127.0.0.1:19090").insecure(true).timeout(2000)
            .retry(retry == null ? RetryConfig.builder().enabled(false).build() : retry)
            .build();
    }

    @Test
    void invokeWithDisconnectedTransportFailsAsUnavailable() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = new InvokerImpl(config(null), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        assertTrue(invoker.isConnected());
        // simulate a dropped transport while the invoker still thinks it is connected
        transport.close();
        InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE, error.getErrorCode());
    }

    @Test
    void retryDisabledShortCircuitsSupplier() {
        AtomicInteger attempts = new AtomicInteger();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            attempts.incrementAndGet();
            throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "down");
        });
        InvokerImpl invoker = new InvokerImpl(config(RetryConfig.builder().enabled(false).build()),
            (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
        assertEquals(1, attempts.get());
    }

    @Test
    void retryRetriesOnlyRetryableCodesAndRespectsMaxAttempts() {
        AtomicInteger attempts = new AtomicInteger();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            attempts.incrementAndGet();
            // UNAVAILABLE maps to 14 which is in the default retryable set
            throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "down");
        });
        InvokerImpl invoker = new InvokerImpl(config(RetryConfig.builder()
                .enabled(true).maxAttempts(3).initialDelayMs(1).maxDelayMs(2).build()),
            (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
        assertEquals(3, attempts.get());

        // INVALID_ARGUMENT (3) is not retryable: exactly one attempt
        AtomicInteger single = new AtomicInteger();
        FakeTransportClient noRetry = new FakeTransportClient((msgType, data) -> {
            single.incrementAndGet();
            throw new InvokerException(InvokerException.ErrorCode.INVALID_ARGUMENT, "bad");
        });
        InvokerImpl strict = new InvokerImpl(config(RetryConfig.builder()
                .enabled(true).maxAttempts(5).initialDelayMs(1).build()),
            (address, timeout) -> noRetry);
        assertDoesNotThrow(strict::connect);
        assertThrows(InvokerException.class, () -> strict.invoke("fn", "{}"));
        assertEquals(1, single.get());
    }

    @Test
    void negativeRetryDelaysAreClampedToZero() {
        AtomicInteger attempts = new AtomicInteger();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            attempts.incrementAndGet();
            throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "down");
        });
        InvokerImpl invoker = new InvokerImpl(config(RetryConfig.builder()
                .enabled(true).maxAttempts(3).initialDelayMs(-500).maxDelayMs(-500)
                .backoffMultiplier(-1).jitterFactor(0).build()),
            (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        long start = System.nanoTime();
        assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
        long elapsedMs = (System.nanoTime() - start) / 1_000_000;
        assertTrue(elapsedMs < 500, "negative delays must clamp to zero, took " + elapsedMs + "ms");
        assertEquals(3, attempts.get());
    }

    @Test
    void startTaskWithEmptyTaskIdFailsInternally() {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) ->
            SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("")));
        InvokerImpl invoker = new InvokerImpl(config(null), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        InvokerException error = assertThrows(InvokerException.class,
            () -> invoker.startTask("fn", null));
        assertTrue(error.getMessage().contains("task ID"));
    }

    @Test
    void pollingStreamCoversTypeNormalizationAndCompletion() throws Exception {
        // START_TASK returns the task id; STREAM polls return progress events with payload
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-stream"));
            }
            return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent(
                "progress", "half", 50, "{\"n\":2}".getBytes(StandardCharsets.UTF_8)));
        });
        InvokerImpl invoker = new InvokerImpl(config(null), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);

        String taskId = invoker.startTask("fn", "{}");
        assertEquals("task-stream", taskId);

        CountDownLatch polled = new CountDownLatch(2);
        List<TaskEventInfo> events = new java.util.ArrayList<>();
        invoker.streamTask(taskId).subscribe(new Subscriber<TaskEventInfo>() {
            @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
            @Override public void onNext(TaskEventInfo event) { events.add(event); polled.countDown(); }
            @Override public void onError(Throwable error) { }
            @Override public void onComplete() { }
        });
        assertTrue(polled.await(5, TimeUnit.SECONDS));
        assertEquals("started", events.get(0).getType());
        assertEquals("progress", events.get(1).getType());
        assertEquals("{\"n\":2}", events.get(1).getPayload());
        assertEquals(50, events.get(1).getProgress());
    }

    @Test
    void cancelledAndErrorTypesNormalizeCorrectly() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-cancel"));
            }
            return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent(
                "error", "user requested cancel", 100, new byte[0]));
        });
        InvokerImpl invoker = new InvokerImpl(config(null), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);

        String taskId = invoker.startTask("fn", "{}");
        CountDownLatch terminal = new CountDownLatch(1);
        List<TaskEventInfo> events = new java.util.ArrayList<>();
        invoker.streamTask(taskId).subscribe(new Subscriber<TaskEventInfo>() {
            @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
            @Override public void onNext(TaskEventInfo event) { events.add(event); }
            @Override public void onError(Throwable error) { terminal.countDown(); }
            @Override public void onComplete() { terminal.countDown(); }
        });
        assertTrue(terminal.await(5, TimeUnit.SECONDS));
        assertTrue(events.size() >= 2);
        assertEquals("started", events.get(0).getType());
        assertEquals("cancelled", events.get(1).getType());
        assertTrue(events.get(1).isDone());
        assertEquals("user requested cancel", events.get(1).getError());
    }

    @Test
    void simulateTaskProgressOnUnknownOrCompletedTaskIsIgnored() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) ->
            SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-sim")));
        InvokerImpl invoker = new InvokerImpl(config(null), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);

        // unknown task: no state, no exception
        assertDoesNotThrow(() -> invoker.simulateTaskProgress("missing", 10, "x"));

        String taskId = invoker.startTask("fn", "{}");
        assertDoesNotThrow(() -> invoker.simulateTaskProgress(taskId, 10, "tenth"));
    }

    @Test
    void getTaskStatusCoversTypeMapping() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) ->
            SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-mapped")));
        InvokerImpl invoker = new InvokerImpl(config(null), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        assertEquals(null, invoker.getTaskStatus("unknown-task"));
        String taskId = invoker.startTask("fn", "{}");
        assertEquals(InvokerImpl.TaskStatus.STARTED, invoker.getTaskStatus(taskId));
    }
}
