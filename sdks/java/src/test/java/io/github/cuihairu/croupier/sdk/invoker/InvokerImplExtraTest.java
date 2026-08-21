package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TCPTransport;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Additional tests for InvokerImpl: retry overrides, interruptions,
 * cancellation errors and task event streaming edge cases.
 */
@DisplayName("InvokerImpl retry and streaming edge cases")
class InvokerImplExtraTest {

    /** Fake transport whose streamed task behavior is selected by mode. */
    private static final class ScriptedTransport implements TransportClient {
        final String taskId;
        final String mode; // done | cancelled | fail | progress
        final FakeTransportClient delegate;

        ScriptedTransport(String taskId, String mode) {
            this.taskId = taskId;
            this.mode = mode;
            this.delegate = new FakeTransportClient(this::respond);
        }

        private byte[] respond(int msgType, byte[] data) throws InvokerException {
            if (msgType == Protocol.MSG_INVOKE_REQUEST) {
                return SdkWireMessages.encodeInvokeResponse(
                    new SdkWireMessages.InvokeResponse("invoke-ok".getBytes(StandardCharsets.UTF_8)));
            }
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse(taskId));
            }
            if (msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                SdkWireMessages.TaskStreamRequest request = SdkWireMessages.decodeTaskStreamRequest(data);
                if (!taskId.equals(request.taskId)) {
                    return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent("progress", "other", 5, new byte[0]));
                }
                return switch (mode) {
                    case "done" -> SdkWireMessages.encodeTaskEvent(
                        new SdkWireMessages.TaskEvent("done", "finished", 100, "res".getBytes(StandardCharsets.UTF_8)));
                    case "cancelled" -> SdkWireMessages.encodeTaskEvent(
                        new SdkWireMessages.TaskEvent("error", "cancelled by admin", 0, new byte[0]));
                    case "fail" -> throw new IllegalStateException("stream broken");
                    default -> SdkWireMessages.encodeTaskEvent(
                        new SdkWireMessages.TaskEvent("progress", "working", 10, new byte[0]));
                };
            }
            return new byte[0];
        }

        @Override
        public void connect() {
            delegate.connect();
        }

        @Override
        public byte[] request(int msgType, byte[] data) throws InvokerException {
            return delegate.request(msgType, data);
        }

        @Override
        public boolean isConnected() {
            return delegate.isConnected();
        }

        @Override
        public void close() {
            delegate.close();
        }
    }

    /** Transport whose requests always fail with a retryable UNAVAILABLE error. */
    private static final class UnavailableTransport implements TransportClient {
        final AtomicInteger requests = new AtomicInteger();
        final FakeTransportClient delegate = new FakeTransportClient((msgType, data) -> {
            requests.incrementAndGet();
            throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "connection down");
        });

        @Override
        public void connect() {
            delegate.connect();
        }

        @Override
        public byte[] request(int msgType, byte[] data) throws InvokerException {
            return delegate.request(msgType, data);
        }

        @Override
        public boolean isConnected() {
            return delegate.isConnected();
        }

        @Override
        public void close() {
            delegate.close();
        }
    }

    private static RetryConfig retry(int attempts, int initialDelayMs) {
        return RetryConfig.builder()
            .enabled(true)
            .maxAttempts(attempts)
            .initialDelayMs(initialDelayMs)
            .maxDelayMs(initialDelayMs)
            .backoffMultiplier(1.0)
            .jitterFactor(0.0)
            .retryableStatusCodes(List.of(14))
            .build();
    }

    private static RetryConfig noRetry() {
        return RetryConfig.builder().enabled(false).build();
    }

    private static InvokerConfig.Builder config(String address) {
        return InvokerConfig.builder().address(address).retry(noRetry());
    }

    @Test
    @DisplayName("single-argument constructor builds a TCP transport factory")
    void singleArgumentConstructor() throws Exception {
        InvokerImpl invoker = new InvokerImpl(config("tcp://127.0.0.1:19091").build());
        assertFalse(invoker.isConnected());
        assertDoesNotThrow(invoker::close);

        java.lang.reflect.Method factory = InvokerImpl.class.getDeclaredMethod("createTransportFactory", InvokerConfig.class);
        factory.setAccessible(true);
        @SuppressWarnings("unchecked")
        java.util.function.BiFunction<String, Integer, TransportClient> created =
            (java.util.function.BiFunction<String, Integer, TransportClient>) factory.invoke(null, InvokerConfig.builder().build());
        assertInstanceOf(TCPTransport.class, created.apply("tcp://example.com:7777", 1500));
        assertInstanceOf(TCPTransport.class, created.apply("example.com", 1500));
    }

    @Test
    @DisplayName("two-argument invoke handles null payloads")
    @Timeout(10)
    void invokeTwoArgumentsNullPayload() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-x", "done");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        assertEquals("invoke-ok", invoker.invoke("f", null));
        assertTrue(invoker.isConnected());
        invoker.close();
        assertFalse(invoker.isConnected());
    }

    @Test
    @DisplayName("two-argument startTask handles null payloads")
    @Timeout(10)
    void startTaskTwoArgumentsNullPayload() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-n", "progress");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        assertEquals("task-n", invoker.startTask("f", null));
        invoker.close();
    }

    @Test
    @DisplayName("stream polling failures with direct invoker exceptions surface as errors")
    @Timeout(10)
    void streamPollingInvokerExceptionSurfaces() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-ie"));
            }
            if (msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "stream unavailable");
            }
            return new byte[0];
        });
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();
        invoker.startTask("f", "{}");

        CountDownLatch errored = new CountDownLatch(1);
        AtomicReference<Throwable> captured = new AtomicReference<>();
        invoker.streamTask("task-ie").subscribe(new Subscriber<TaskEventInfo>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }

            @Override
            public void onNext(TaskEventInfo event) {
            }

            @Override
            public void onError(Throwable error) {
                captured.set(error);
                errored.countDown();
            }

            @Override
            public void onComplete() {
            }
        });
        assertTrue(errored.await(5, TimeUnit.SECONDS));
        assertInstanceOf(InvokerException.class, captured.get());
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE, ((InvokerException) captured.get()).getErrorCode());
        invoker.close();
    }

    @Test
    @DisplayName("connecting again closes the previous transport")
    @Timeout(10)
    void connectClosesPreviousTransport() throws Exception {
        List<ScriptedTransport> created = new CopyOnWriteArrayList<>();
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> {
            ScriptedTransport transport = new ScriptedTransport("task-x", "done");
            created.add(transport);
            return transport;
        });
        invoker.connect();
        ScriptedTransport first = created.get(0);
        assertTrue(first.isConnected());

        java.lang.reflect.Field field = InvokerImpl.class.getDeclaredField("connected");
        field.setAccessible(true);
        field.setBoolean(invoker, false);
        invoker.connect();

        assertEquals(2, created.size());
        assertFalse(first.isConnected());
        assertTrue(created.get(1).isConnected());
        invoker.close();
    }

    @Test
    @DisplayName("per-call retry override disables config-level retry")
    @Timeout(10)
    void retryOverrideDisablesRetry() throws Exception {
        UnavailableTransport transport = new UnavailableTransport();
        InvokerImpl invoker = new InvokerImpl(InvokerConfig.builder()
            .address("agent")
            .retry(retry(3, 1))
            .build(), (address, timeout) -> transport);
        invoker.connect();

        InvokerException error = assertThrows(InvokerException.class, () ->
            invoker.invoke("f", "{}", InvokeOptions.builder().retry(noRetry()).build()));
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE, error.getErrorCode());
        assertEquals(1, transport.requests.get(), "disabled retry must not repeat the request");
        invoker.close();
    }

    @Test
    @DisplayName("config retry repeats retryable failures until attempts are exhausted")
    @Timeout(10)
    void configRetryExhaustsAttempts() throws Exception {
        UnavailableTransport transport = new UnavailableTransport();
        InvokerImpl invoker = new InvokerImpl(InvokerConfig.builder()
            .address("agent")
            .retry(retry(3, 1))
            .build(), (address, timeout) -> transport);
        invoker.connect();

        InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("f", "{}"));
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE, error.getErrorCode());
        assertEquals(3, transport.requests.get(), "expected three attempts");
        invoker.close();
    }

    @Test
    @DisplayName("interrupting a retry backoff cancels the operation")
    @Timeout(10)
    void interruptedRetryCancels() throws Exception {
        UnavailableTransport transport = new UnavailableTransport();
        InvokerImpl invoker = new InvokerImpl(InvokerConfig.builder()
            .address("agent")
            .retry(retry(5, 10_000))
            .build(), (address, timeout) -> transport);
        invoker.connect();

        AtomicReference<Throwable> captured = new AtomicReference<>();
        Thread caller = new Thread(() -> {
            try {
                invoker.invoke("f", "{}");
            } catch (Throwable error) {
                captured.set(error);
            }
        });
        caller.start();
        Thread.sleep(200);
        caller.interrupt();
        caller.join(3000);

        Throwable error = captured.get();
        assertInstanceOf(InvokerException.class, error);
        assertEquals(InvokerException.ErrorCode.CANCELLED, ((InvokerException) error).getErrorCode());
        assertTrue(error.getMessage().contains("interrupted"), error.getMessage());
        invoker.close();
    }

    @Test
    @DisplayName("cancelTask rethrows invoker failures and wraps runtime failures")
    @Timeout(10)
    void cancelTaskErrorMapping() throws Exception {
        FakeTransportClient denied = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-c"));
            }
            if (msgType == Protocol.MSG_CANCEL_TASK_REQUEST) {
                throw new InvokerException(InvokerException.ErrorCode.PERMISSION_DENIED, "not allowed");
            }
            return new byte[0];
        });
        InvokerImpl deniedInvoker = new InvokerImpl(config("agent").build(), (address, timeout) -> denied);
        deniedInvoker.connect();
        deniedInvoker.startTask("f", "{}");
        InvokerException mapped = assertThrows(InvokerException.class, () -> deniedInvoker.cancelTask("task-c"));
        assertEquals(InvokerException.ErrorCode.PERMISSION_DENIED, mapped.getErrorCode());
        deniedInvoker.close();

        FakeTransportClient runtime = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-r"));
            }
            if (msgType == Protocol.MSG_CANCEL_TASK_REQUEST) {
                throw new IllegalStateException("boom");
            }
            return new byte[0];
        });
        InvokerImpl runtimeInvoker = new InvokerImpl(config("agent").build(), (address, timeout) -> runtime);
        runtimeInvoker.connect();
        runtimeInvoker.startTask("f", "{}");
        InvokerException wrapped = assertThrows(InvokerException.class, () -> runtimeInvoker.cancelTask("task-r"));
        assertEquals(InvokerException.ErrorCode.INTERNAL, wrapped.getErrorCode());
        assertTrue(wrapped.getMessage().contains("CancelTask failed"), wrapped.getMessage());
        runtimeInvoker.close();
    }

    @Test
    @DisplayName("cancelTask rejects unknown and already finished tasks")
    @Timeout(10)
    void cancelTaskUnknownAndFinished() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-x", "done");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();

        InvokerException unknown = assertThrows(InvokerException.class, () -> invoker.cancelTask("ghost"));
        assertEquals(InvokerException.ErrorCode.NOT_FOUND, unknown.getErrorCode());

        invoker.startTask("f", "{}");
        CollectingSubscriber subscriber = new CollectingSubscriber();
        invoker.streamTask("task-x").subscribe(subscriber);
        assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS));

        InvokerException finished = assertThrows(InvokerException.class, () -> invoker.cancelTask("task-x"));
        assertEquals(InvokerException.ErrorCode.FAILED_PRECONDITION, finished.getErrorCode());
        invoker.close();
    }

    @Test
    @DisplayName("streamTask errors for unknown tasks")
    @Timeout(10)
    void streamTaskUnknownTask() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-x", "done");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();

        CountDownLatch errored = new CountDownLatch(1);
        AtomicReference<Throwable> captured = new AtomicReference<>();
        invoker.streamTask("missing").subscribe(new Subscriber<TaskEventInfo>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }

            @Override
            public void onNext(TaskEventInfo event) {
                captured.set(new AssertionError("unexpected event"));
            }

            @Override
            public void onError(Throwable error) {
                captured.set(error);
                errored.countDown();
            }

            @Override
            public void onComplete() {
                captured.set(new AssertionError("unexpected completion"));
            }
        });
        assertTrue(errored.await(5, TimeUnit.SECONDS));
        assertInstanceOf(InvokerException.class, captured.get());
        assertTrue(captured.get().getMessage().contains("Task not found"), captured.get().getMessage());
        invoker.close();
    }

    @Test
    @DisplayName("done events are normalized to completed and complete the stream")
    @Timeout(10)
    void streamDoneNormalized() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-x", "done");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();
        assertEquals("task-x", invoker.startTask("f", "{}"));

        CollectingSubscriber subscriber = new CollectingSubscriber();
        invoker.streamTask("task-x").subscribe(subscriber);
        assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS));
        assertTrue(subscriber.types.contains("started"));
        assertTrue(subscriber.types.contains("completed"));
        invoker.close();
    }

    @Test
    @DisplayName("error events carrying cancel messages are normalized to cancelled")
    @Timeout(10)
    void streamCancelMessageNormalized() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-c", "cancelled");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();
        invoker.startTask("f", "{}");

        CollectingSubscriber subscriber = new CollectingSubscriber();
        invoker.streamTask("task-c").subscribe(subscriber);
        assertTrue(subscriber.completed.await(5, TimeUnit.SECONDS));
        assertTrue(subscriber.types.contains("cancelled"), subscriber.types.toString());
        assertEquals("cancelled by admin", subscriber.lastEventError);
        invoker.close();
    }

    @Test
    @DisplayName("stream polling failures surface as subscriber errors")
    @Timeout(10)
    void streamPollingFailureSurfaces() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-f", "fail");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();
        invoker.startTask("f", "{}");

        CountDownLatch errored = new CountDownLatch(1);
        AtomicReference<Throwable> captured = new AtomicReference<>();
        invoker.streamTask("task-f").subscribe(new Subscriber<TaskEventInfo>() {
            @Override
            public void onSubscribe(Subscription subscription) {
                subscription.request(Long.MAX_VALUE);
            }

            @Override
            public void onNext(TaskEventInfo event) {
            }

            @Override
            public void onError(Throwable error) {
                captured.set(error);
                errored.countDown();
            }

            @Override
            public void onComplete() {
            }
        });
        assertTrue(errored.await(5, TimeUnit.SECONDS));
        assertInstanceOf(InvokerException.class, captured.get());
        assertTrue(captured.get().getMessage().contains("StreamTask failed"), captured.get().getMessage());
        invoker.close();
    }

    @Test
    @DisplayName("progress polling continues while the task is unfinished")
    @Timeout(10)
    void streamProgressKeepsPolling() throws Exception {
        ScriptedTransport transport = new ScriptedTransport("task-p", "progress");
        InvokerImpl invoker = new InvokerImpl(config("agent").build(), (address, timeout) -> transport);
        invoker.connect();
        invoker.startTask("f", "{}");

        CollectingSubscriber subscriber = new CollectingSubscriber();
        invoker.streamTask("task-p").subscribe(subscriber);
        assertTrue(subscriber.firstEvent.await(5, TimeUnit.SECONDS));
        assertEquals("started", subscriber.types.get(0));
        invoker.close();
    }

    private static final class CollectingSubscriber implements Subscriber<TaskEventInfo> {
        final List<String> types = new CopyOnWriteArrayList<>();
        final CountDownLatch firstEvent = new CountDownLatch(1);
        final CountDownLatch completed = new CountDownLatch(1);
        volatile String lastSubscriberError;
        volatile String lastEventError;

        @Override
        public void onSubscribe(Subscription subscription) {
            subscription.request(Long.MAX_VALUE);
        }

        @Override
        public void onNext(TaskEventInfo event) {
            types.add(event.getType());
            lastEventError = event.getError();
            firstEvent.countDown();
        }

        @Override
        public void onError(Throwable error) {
            lastSubscriberError = error.getMessage();
            completed.countDown();
        }

        @Override
        public void onComplete() {
            completed.countDown();
        }
    }
}
