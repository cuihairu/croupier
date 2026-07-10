package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;
import static org.junit.jupiter.api.Assertions.*;

class InvokerImplTest {

    private InvokerConfig config;

    @BeforeEach
    void setUp() {
        config = InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .build();
    }

    @Test
    @DisplayName("connect() should connect transport")
    void testConnect() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        invoker.connect();

        assertTrue(invoker.isConnected());
        assertTrue(transport.isConnected());
    }

    @Test
    @DisplayName("invoke() should send protobuf request and return protobuf response")
    void testInvoke() throws InvokerException {
        AtomicReference<SdkWireMessages.InvokeRequest> captured = new AtomicReference<>();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            assertEquals(Protocol.MSG_INVOKE_REQUEST, msgType);
            captured.set(SdkWireMessages.decodeInvokeRequest(data));
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("{\"ok\":true}".getBytes(StandardCharsets.UTF_8))
            );
        });
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke(
            "player.ban",
            "{\"player_id\":\"123\"}",
            InvokeOptions.builder()
                .idempotencyKey("idem-1")
                .headers(Map.of("X-Request-ID", "req-1"))
                .build()
        );

        assertEquals("{\"ok\":true}", result);
        assertNotNull(captured.get());
        assertEquals("player.ban", captured.get().functionId);
        assertEquals("idem-1", captured.get().idempotencyKey);
        assertEquals("{\"player_id\":\"123\"}", new String(captured.get().payload, StandardCharsets.UTF_8));
        assertEquals("req-1", captured.get().metadata.get("X-Request-ID"));
    }

    @Test
    @DisplayName("startTask() should create tracked task from transport response")
    void testStartTask() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            assertEquals(Protocol.MSG_START_TASK_REQUEST, msgType);
            return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-123"));
        });
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String taskId = invoker.startTask("player.sync", "{\"user\":\"u1\"}");

        assertEquals("task-123", taskId);
        assertTrue(invoker.hasTask(taskId));
        assertEquals(InvokerImpl.TaskStatus.STARTED, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("streamTask() should poll until terminal event and normalize done to completed")
    void testStreamTask() throws Exception {
        AtomicInteger streamCalls = new AtomicInteger();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-1"));
            }
            if (msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                int call = streamCalls.getAndIncrement();
                if (call == 0) {
                    return SdkWireMessages.encodeTaskEvent(
                        new SdkWireMessages.TaskEvent("progress", "working", 50, new byte[0])
                    );
                }
                return SdkWireMessages.encodeTaskEvent(
                    new SdkWireMessages.TaskEvent(
                        "done",
                        "finished",
                        100,
                        "{\"result\":1}".getBytes(StandardCharsets.UTF_8)
                    )
                );
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);
        String taskId = invoker.startTask("player.sync", "{}");

        List<TaskEventInfo> events = new ArrayList<>();
        CountDownLatch latch = new CountDownLatch(1);
        invoker.streamTask(taskId).subscribe(new Subscriber<>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }

            @Override
            public void onNext(TaskEventInfo event) {
                events.add(event);
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

        assertTrue(latch.await(5, TimeUnit.SECONDS));
        assertEquals(3, events.size());
        assertEquals("started", events.get(0).getType());
        assertEquals("progress", events.get(1).getType());
        assertEquals("completed", events.get(2).getType());
        assertEquals("{\"result\":1}", events.get(2).getPayload());
        assertTrue(events.get(2).isDone());
        assertEquals(InvokerImpl.TaskStatus.COMPLETED, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("cancelTask() should send cancel request and mark task cancelled")
    void testCancelTask() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-9"));
            }
            if (msgType == Protocol.MSG_CANCEL_TASK_REQUEST) {
                SdkWireMessages.CancelTaskRequest request = SdkWireMessages.decodeCancelTaskRequest(data);
                assertEquals("task-9", request.taskId);
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);
        String taskId = invoker.startTask("player.sync", "{}");

        invoker.cancelTask(taskId);

        assertEquals(InvokerImpl.TaskStatus.CANCELLED, invoker.getTaskStatus(taskId));
    }

    @Test
    @DisplayName("streamTask() should error for unknown task id")
    void testStreamTaskUnknownTask() throws Exception {
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> new FakeTransportClient((msgType, data) -> new byte[0]));
        AtomicReference<Throwable> error = new AtomicReference<>();
        CountDownLatch latch = new CountDownLatch(1);

        invoker.streamTask("missing").subscribe(new Subscriber<>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(1);
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
                latch.countDown();
            }
        });

        assertTrue(latch.await(2, TimeUnit.SECONDS));
        assertInstanceOf(InvokerException.class, error.get());
        assertEquals(ErrorCode.NOT_FOUND, ((InvokerException) error.get()).getErrorCode());
    }

    @Test
    @DisplayName("null config should surface wrapped NPE on connect")
    void testNullConfig() {
        InvokerImpl invoker = new InvokerImpl(null, (address, timeout) -> new FakeTransportClient((msgType, data) -> new byte[0]));

        InvokerException exception = assertThrows(InvokerException.class, invoker::connect);
        assertInstanceOf(NullPointerException.class, exception.getCause());
        assertEquals(ErrorCode.CONNECTION_FAILED, exception.getErrorCode());
    }
}
