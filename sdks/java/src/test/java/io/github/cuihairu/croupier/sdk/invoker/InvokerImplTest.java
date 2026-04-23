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
    @DisplayName("startJob() should create tracked job from transport response")
    void testStartJob() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            assertEquals(Protocol.MSG_START_JOB_REQUEST, msgType);
            return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-123"));
        });
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String jobId = invoker.startJob("player.sync", "{\"user\":\"u1\"}");

        assertEquals("job-123", jobId);
        assertTrue(invoker.hasJob(jobId));
        assertEquals(InvokerImpl.JobStatus.STARTED, invoker.getJobStatus(jobId));
    }

    @Test
    @DisplayName("streamJob() should poll until terminal event and normalize done to completed")
    void testStreamJob() throws Exception {
        AtomicInteger streamCalls = new AtomicInteger();
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            if (msgType == Protocol.MSG_STREAM_JOB_REQUEST) {
                int call = streamCalls.getAndIncrement();
                if (call == 0) {
                    return SdkWireMessages.encodeJobEvent(
                        new SdkWireMessages.JobEvent("progress", "working", 50, new byte[0])
                    );
                }
                return SdkWireMessages.encodeJobEvent(
                    new SdkWireMessages.JobEvent(
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
        String jobId = invoker.startJob("player.sync", "{}");

        List<JobEventInfo> events = new ArrayList<>();
        CountDownLatch latch = new CountDownLatch(1);
        invoker.streamJob(jobId).subscribe(new Subscriber<>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }

            @Override
            public void onNext(JobEventInfo event) {
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
        assertEquals(InvokerImpl.JobStatus.COMPLETED, invoker.getJobStatus(jobId));
    }

    @Test
    @DisplayName("cancelJob() should send cancel request and mark job cancelled")
    void testCancelJob() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-9"));
            }
            if (msgType == Protocol.MSG_CANCEL_JOB_REQUEST) {
                SdkWireMessages.CancelJobRequest request = SdkWireMessages.decodeCancelJobRequest(data);
                assertEquals("job-9", request.jobId);
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);
        String jobId = invoker.startJob("player.sync", "{}");

        invoker.cancelJob(jobId);

        assertEquals(InvokerImpl.JobStatus.CANCELLED, invoker.getJobStatus(jobId));
    }

    @Test
    @DisplayName("streamJob() should error for unknown job id")
    void testStreamJobUnknownJob() throws Exception {
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> new FakeTransportClient((msgType, data) -> new byte[0]));
        AtomicReference<Throwable> error = new AtomicReference<>();
        CountDownLatch latch = new CountDownLatch(1);

        invoker.streamJob("missing").subscribe(new Subscriber<>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(1);
            }

            @Override
            public void onNext(JobEventInfo event) {
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
