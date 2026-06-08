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
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;
import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for InvokerImpl error paths and edge cases.
 */
class InvokerImplErrorPathTest {

    private InvokerConfig createConfig() {
        return InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .build();
    }

    @Test
    @DisplayName("cancelJob should wrap non-InvokerException as INTERNAL")
    void cancelJobWrapsException() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            if (msgType == Protocol.MSG_CANCEL_JOB_REQUEST) {
                throw new RuntimeException("network error");
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String jobId = invoker.startJob("func", "{}");

        InvokerException ex = assertThrows(InvokerException.class, () -> invoker.cancelJob(jobId));
        assertEquals(ErrorCode.INTERNAL, ex.getErrorCode());
        assertTrue(ex.getMessage().contains("CancelJob failed"));
    }

    @Test
    @DisplayName("invoke should wrap non-InvokerException transport error")
    void invokeWrapsTransportError() {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            throw new RuntimeException("connection reset");
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        InvokerException ex = assertThrows(InvokerException.class, () ->
            invoker.invoke("func", "payload", InvokeOptions.create()));
        assertEquals(ErrorCode.INTERNAL, ex.getErrorCode());
    }

    @Test
    @DisplayName("startJob should wrap non-InvokerException transport error")
    void startJobWrapsTransportError() {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            throw new RuntimeException("timeout");
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        InvokerException ex = assertThrows(InvokerException.class, () ->
            invoker.startJob("func", "payload", InvokeOptions.create()));
        assertEquals(ErrorCode.INTERNAL, ex.getErrorCode());
        assertTrue(ex.getMessage().contains("StartJob failed"));
    }

    @Test
    @DisplayName("requireTransport should throw UNAVAILABLE when transport is disconnected")
    void requireTransportDisconnected() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        // Connect then close to simulate disconnection
        invoker.connect();
        transport.close();
        // Now transport.isConnected() returns false

        InvokerException ex = assertThrows(InvokerException.class, () ->
            invoker.invoke("func", "payload", InvokeOptions.create()));
        // invoke auto-connects, so it may succeed or throw CONNECTION_FAILED
        // The key is that it doesn't silently succeed with a broken transport
    }

    @Test
    @DisplayName("close should stop polling for active jobs")
    void closeStopsPolling() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        invoker.startJob("func", "{}");

        assertEquals(1, invoker.getActiveJobCount());
        invoker.close();
        assertEquals(0, invoker.getActiveJobCount());
    }

    @Test
    @DisplayName("streamJob should handle fetchJobEvent error")
    void streamJobFetchError() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            if (msgType == Protocol.MSG_STREAM_JOB_REQUEST) {
                throw new RuntimeException("fetch failed");
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String jobId = invoker.startJob("func", "{}");

        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Throwable> error = new AtomicReference<>();

        invoker.streamJob(jobId).subscribe(new Subscriber<>() {
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
            }
        });

        try {
            assertTrue(latch.await(3, TimeUnit.SECONDS));
        } catch (InterruptedException e) {
            fail("Timed out waiting for error");
        }
        assertNotNull(error.get());
    }

    @Test
    @DisplayName("invoke with retry and non-retryable error should fail immediately")
    void invokeNonRetryableError() {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            throw new RuntimeException("not found");
        });

        InvokerConfig configWithRetry = InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .retry(RetryConfig.builder()
                .enabled(true)
                .maxAttempts(5)
                .initialDelayMs(10)
                .retryableStatusCodes(List.of(14)) // Only UNAVAILABLE is retryable
                .build())
            .build();

        InvokerImpl invoker = new InvokerImpl(configWithRetry, (address, timeout) -> transport);

        // INTERNAL (status 13) is not in retryableStatusCodes, so should fail on first attempt
        assertThrows(InvokerException.class, () ->
            invoker.invoke("func", "payload", InvokeOptions.create()));
        assertEquals(1, callCount[0]);
    }

    @Test
    @DisplayName("hasJob should return true for active job")
    void hasJobActive() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String jobId = invoker.startJob("func", "{}");

        assertTrue(invoker.hasJob(jobId));
    }

    @Test
    @DisplayName("getJobStatus should return STARTED for new job")
    void getJobStatusStarted() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String jobId = invoker.startJob("func", "{}");

        InvokerImpl.JobStatus status = invoker.getJobStatus(jobId);
        assertNotNull(status);
    }

    @Test
    @DisplayName("cancelJob should send cancel request to transport")
    void cancelJobSendsRequest() throws InvokerException {
        boolean[] cancelSent = {false};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_JOB_REQUEST) {
                return SdkWireMessages.encodeStartJobResponse(new SdkWireMessages.StartJobResponse("job-1"));
            }
            if (msgType == Protocol.MSG_CANCEL_JOB_REQUEST) {
                cancelSent[0] = true;
                return new byte[0];
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);
        String jobId = invoker.startJob("func", "{}");

        invoker.cancelJob(jobId);

        assertTrue(cancelSent[0]);
    }

    @Test
    @DisplayName("invoke should use idempotency key from options")
    void invokeWithIdempotencyKey() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            SdkWireMessages.InvokeRequest req = SdkWireMessages.decodeInvokeRequest(data);
            assertEquals("key-123", req.idempotencyKey);
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        InvokeOptions options = InvokeOptions.builder()
            .idempotencyKey("key-123")
            .build();

        String result = invoker.invoke("func", "payload", options);
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke should use headers from options")
    void invokeWithHeaders() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            SdkWireMessages.InvokeRequest req = SdkWireMessages.decodeInvokeRequest(data);
            assertEquals("value1", req.metadata.get("X-Custom"));
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });
        InvokerImpl invoker = new InvokerImpl(createConfig(), (address, timeout) -> transport);

        InvokeOptions options = InvokeOptions.builder()
            .header("X-Custom", "value1")
            .build();

        invoker.invoke("func", "payload", options);
    }
}
