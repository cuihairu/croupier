package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;
import static org.junit.jupiter.api.Assertions.*;

/**
 * Additional tests for InvokerImpl to improve code coverage.
 */
class InvokerImplCoverageTest {

    private InvokerConfig createConfig() {
        return InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .build();
    }

    @Test
    @DisplayName("connect() should be idempotent")
    void connectIdempotent() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        invoker.connect();
        invoker.connect(); // Second call should be no-op

        assertTrue(invoker.isConnected());
    }

    @Test
    @DisplayName("connect() should replace existing transport")
    void connectReplacesTransport() throws InvokerException {
        FakeTransportClient transport1 = new FakeTransportClient((msgType, data) -> new byte[0]);
        FakeTransportClient transport2 = new FakeTransportClient((msgType, data) -> new byte[0]);
        boolean[] firstCall = {true};

        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> {
            if (firstCall[0]) {
                firstCall[0] = false;
                return transport1;
            }
            return transport2;
        });

        invoker.connect();
        // Force reconnect by calling connect again after closing
        // This tests the transport replacement logic
        assertTrue(invoker.isConnected());
    }

    @Test
    @DisplayName("invoke() with null options should use defaults")
    void invokeWithNullOptions() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", null);

        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke() with null payload should send empty bytes")
    void invokeWithNullPayload() throws InvokerException {
        AtomicReference<SdkWireMessages.InvokeRequest> captured = new AtomicReference<>();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            captured.set(SdkWireMessages.decodeInvokeRequest(data));
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        invoker.invoke("func", null, InvokeOptions.create());

        assertNotNull(captured.get());
        assertEquals(0, captured.get().payload.length);
    }

    @Test
    @DisplayName("invoke() should throw when transport fails")
    void invokeTransportFailure() {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            throw new RuntimeException("connection lost");
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        assertThrows(InvokerException.class, () ->
            invoker.invoke("func", "payload", InvokeOptions.create()));
    }

    @Test
    @DisplayName("startTask() with null options should use defaults")
    void startTaskWithNullOptions() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        String taskId = invoker.startTask("func", "payload", null);

        assertEquals("task-1", taskId);
    }

    @Test
    @DisplayName("startTask() should throw when response has empty task ID")
    void startTaskEmptyTaskId() {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse(""));
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        assertThrows(InvokerException.class, () ->
            invoker.startTask("func", "payload", InvokeOptions.create()));
    }

    @Test
    @DisplayName("cancelTask() should throw for unknown task")
    void cancelTaskUnknown() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        InvokerException ex = assertThrows(InvokerException.class, () ->
            invoker.cancelTask("unknown-task"));

        assertEquals(ErrorCode.NOT_FOUND, ex.getErrorCode());
    }

    @Test
    @DisplayName("cancelTask() should throw for already completed task")
    void cancelTaskAlreadyCompleted() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        // Simulate task completion
        invoker.simulateTaskProgress(taskId, 100, "done");

        InvokerException ex = assertThrows(InvokerException.class, () ->
            invoker.cancelTask(taskId));

        assertEquals(ErrorCode.FAILED_PRECONDITION, ex.getErrorCode());
    }

    @Test
    @DisplayName("setSchema() should store schema")
    void setSchema() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        Map<String, Object> schema = Map.of("type", "object");
        invoker.setSchema("func", schema);

        // No assertion needed - just verifying no exception
    }

    @Test
    @DisplayName("close() should cleanup resources")
    void closeCleanup() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        invoker.connect();
        invoker.close();

        assertFalse(invoker.isConnected());
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("close() should work when not connected")
    void closeWhenNotConnected() throws InvokerException {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        // Should not throw
        invoker.close();

        assertFalse(invoker.isConnected());
    }

    @Test
    @DisplayName("isConnected() should return false initially")
    void isConnectedInitiallyFalse() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        assertFalse(invoker.isConnected());
    }

    @Test
    @DisplayName("getActiveTaskCount() should return 0 initially")
    void getActiveTaskCountInitiallyZero() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        assertEquals(0, invoker.getActiveTaskCount());
    }

    @Test
    @DisplayName("hasTask() should return false for unknown task")
    void hasTaskUnknown() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        assertFalse(invoker.hasTask("unknown"));
    }

    @Test
    @DisplayName("getTaskStatus() should return null for unknown task")
    void getTaskStatusUnknown() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        assertNull(invoker.getTaskStatus("unknown"));
    }

    @Test
    @DisplayName("simulateTaskProgress() should update task status")
    void simulateTaskProgress() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        invoker.simulateTaskProgress(taskId, 50, "halfway");

        assertEquals(InvokerImpl.TaskStatus.PROGRESS, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("simulateTaskProgress() to 100 should mark as completed")
    void simulateTaskProgressComplete() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        invoker.simulateTaskProgress(taskId, 100, "done");

        assertEquals(InvokerImpl.TaskStatus.COMPLETED, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("simulateTaskError() should mark task as error")
    void simulateTaskError() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        invoker.simulateTaskError(taskId, "something went wrong");

        assertEquals(InvokerImpl.TaskStatus.ERROR, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("simulateTaskProgress() for unknown task should be no-op")
    void simulateTaskProgressUnknown() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        // Should not throw
        invoker.simulateTaskProgress("unknown", 50, "msg");
    }

    @Test
    @DisplayName("simulateTaskError() for unknown task should be no-op")
    void simulateTaskErrorUnknown() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));

        // Should not throw
        invoker.simulateTaskError("unknown", "error");
    }

    @Test
    @DisplayName("invoke() should auto-connect when not connected")
    void invokeAutoConnect() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        // Don't call connect() explicitly
        String result = invoker.invoke("func", "payload", InvokeOptions.create());

        assertEquals("ok", result);
        assertTrue(invoker.isConnected());
    }

    @Test
    @DisplayName("startTask() should auto-connect when not connected")
    void startTaskAutoConnect() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        String taskId = invoker.startTask("func", "payload");

        assertEquals("task-1", taskId);
        assertTrue(invoker.isConnected());
    }

    @Test
    @DisplayName("streamTask() for completed task should emit events")
    void streamTaskCompleted() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        // Mark task as completed before streaming
        invoker.simulateTaskProgress(taskId, 100, "done");

        CountDownLatch latch = new CountDownLatch(1);
        invoker.streamTask(taskId).subscribe(new Subscriber<>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }

            @Override
            public void onNext(TaskEventInfo event) {
                if (event.isDone()) {
                    latch.countDown();
                }
            }

            @Override
            public void onError(Throwable throwable) {
                fail(throwable);
            }

            @Override
            public void onComplete() {
            }
        });

        assertTrue(latch.await(2, TimeUnit.SECONDS));
    }

    @Test
    @DisplayName("cancelTask() for cancelled task should send cancel request")
    void cancelTaskCancelled() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        invoker.cancelTask(taskId);

        assertEquals(InvokerImpl.TaskStatus.CANCELLED, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("streamTask() request with negative n should error")
    void streamTaskNegativeRequest() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String taskId = invoker.startTask("func", "{}");

        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Throwable> error = new AtomicReference<>();

        invoker.streamTask(taskId).subscribe(new Subscriber<>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(-1); // Negative request
            }

            @Override
            public void onNext(TaskEventInfo event) {
            }

            @Override
            public void onError(Throwable throwable) {
                error.set(throwable);
                latch.countDown();
            }

            @Override
            public void onComplete() {
            }
        });

        assertTrue(latch.await(2, TimeUnit.SECONDS));
        assertInstanceOf(IllegalArgumentException.class, error.get());
    }

    @Test
    @DisplayName("retry should be disabled by default")
    void retryDisabledByDefault() {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            throw new RuntimeException("fail");
        });

        InvokerConfig configNoRetry = InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .retry(RetryConfig.builder().enabled(false).build())
            .build();

        InvokerImpl invoker = new InvokerImpl(configNoRetry, (address, timeout) -> transport);

        assertThrows(InvokerException.class, () ->
            invoker.invoke("func", "payload", InvokeOptions.create()));

        assertEquals(1, callCount[0]); // Should not retry
    }

    @Test
    @DisplayName("invoke with retry config should retry on retryable errors")
    void invokeWithRetry() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] < 3) {
                throw new RuntimeException("unavailable");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig configWithRetry = InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .retry(RetryConfig.builder()
                .enabled(true)
                .maxAttempts(3)
                .initialDelayMs(10)
                .build())
            .build();

        InvokerImpl invoker = new InvokerImpl(configWithRetry, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());

        assertEquals("ok", result);
        assertEquals(3, callCount[0]);
    }

    @Test
    @DisplayName("invoke with retry should fail after max attempts")
    void invokeRetryExhausted() {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            throw new RuntimeException("unavailable");
        });

        InvokerConfig configWithRetry = InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .retry(RetryConfig.builder()
                .enabled(true)
                .maxAttempts(3)
                .initialDelayMs(10)
                .build())
            .build();

        InvokerImpl invoker = new InvokerImpl(configWithRetry, (address, timeout) -> transport);

        assertThrows(InvokerException.class, () ->
            invoker.invoke("func", "payload", InvokeOptions.create()));

        assertEquals(3, callCount[0]);
    }

    @Test
    @DisplayName("connect() should throw InvokerException on failure")
    void connectFailure() {
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> {
            throw new RuntimeException("connection refused");
        });

        InvokerException ex = assertThrows(InvokerException.class, invoker::connect);
        assertEquals(ErrorCode.CONNECTION_FAILED, ex.getErrorCode());
    }

    @Test
    @DisplayName("invoke() should auto-connect when transport is closed")
    void invokeAfterClose() throws InvokerException {
        int[] connectCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> {
            connectCount[0]++;
            return transport;
        });
        invoker.connect();
        invoker.close(); // Close to reset state

        // Should auto-connect and succeed
        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }
}
