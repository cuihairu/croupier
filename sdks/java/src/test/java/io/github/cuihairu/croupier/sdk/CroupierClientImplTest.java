package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.TaskEventInfo;
import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.zip.GZIPInputStream;

import static org.junit.jupiter.api.Assertions.*;

class CroupierClientImplTest {

    private ClientConfig config;
    private TestHarness harness;
    private CroupierClientImpl client;

    @BeforeEach
    void setUp() {
        config = new ClientConfig("game-1", "svc-1");
        harness = new TestHarness();
        client = new CroupierClientImpl(
            config,
            (address, timeout) -> harness.newTransport()
        );
    }

    @Test
    void constructorWithNullConfigThrowsException() {
        assertThrows(NullPointerException.class, () -> new CroupierClientImpl(
            null,
            (address, timeout) -> harness.newTransport()
        ));
    }

    @Test
    void connectFailsWithoutRegisteredFunctions() {
        RuntimeException exception = assertThrows(RuntimeException.class, () -> client.connect().join());
        assertTrue(exception.getCause() instanceof CroupierException);
    }

    @Test
    void connectRegistersLocalFunctions() throws Exception {
        FunctionDescriptor descriptor = new FunctionDescriptor("player.ban", "1.0.0");
        descriptor.setSummary("Ban player");
        descriptor.setInputSchema("{\"type\":\"object\"}");
        client.registerFunction(descriptor, (ctx, payload) -> "{\"ok\":true}");

        client.connect().join();

        assertTrue(client.isConnected());
        assertNotNull(harness.registerRequest.get());
        assertEquals("svc-1", harness.registerRequest.get().serviceId);
        assertEquals(1, harness.registerRequest.get().functions.size());
        assertEquals("player.ban", harness.registerRequest.get().functions.get(0).id);
        assertEquals("Ban player", harness.registerRequest.get().functions.get(0).summary);
    }

    @Test
    void registerFunctionAfterConnectThrows() throws Exception {
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor("f2", "1.0.0"), (ctx, payload) -> "ok"));
    }

    @Test
    void invokeCallsRegisteredHandler() throws CroupierException {
        client.registerFunction(new FunctionDescriptor("test.fn", "1.0.0"), (ctx, payload) -> "result:" + payload);

        String result = client.invoke("test.fn", "input", Map.of("key", "value"));

        assertEquals("result:input", result);
    }

    @Test
    void startTaskDelegatesToInvokerTransport() throws Exception {
        client.registerFunction(new FunctionDescriptor("local.fn", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        String taskId = client.startTask("remote.fn", "{}", Map.of("X-Trace", "trace-1"));

        assertEquals("remote-task-1", taskId);
        assertEquals("trace-1", harness.lastRemoteInvoke.get().metadata.get("X-Trace"));
    }

    @Test
    void streamTaskReturnsCompletedEventFromInvoker() throws Exception {
        client.registerFunction(new FunctionDescriptor("local.fn", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();
        String taskId = client.startTask("remote.fn", "{}");

        List<TaskEventInfo> events = new ArrayList<>();
        CountDownLatch latch = new CountDownLatch(1);
        client.streamTask(taskId).subscribe(new Subscriber<>() {
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
        assertEquals("completed", events.get(events.size() - 1).getType());
        assertEquals("{\"ok\":1}", events.get(events.size() - 1).getPayload());
    }

    @Test
    void heartbeatIsSentAfterConnect() throws Exception {
        config.setHeartbeatInterval(1);
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        client.connect().join();

        assertTrue(harness.heartbeatLatch.await(3, TimeUnit.SECONDS));
    }

    @Test
    void buildManifestIncludesExtendedFields() throws Exception {
        FunctionDescriptor descriptor = new FunctionDescriptor("full.fn", "1.2.0");
        descriptor.setTags(List.of("player", "moderation"));
        descriptor.setSummary("Summary");
        descriptor.setDescription("Description");
        descriptor.setOperationId("playerBan");
        descriptor.setDeprecated(true);
        descriptor.setInputSchema("{\"type\":\"object\"}");
        descriptor.setOutputSchema("{\"type\":\"object\"}");
        descriptor.setCategory("game");
        descriptor.setRisk("danger");
        descriptor.setEntity("Player");
        descriptor.setOperation("update");
        client.registerFunction(descriptor, (ctx, payload) -> "ok");

        String json = new String(client.buildManifest(), StandardCharsets.UTF_8);

        assertTrue(json.contains("\"summary\":\"Summary\""));
        assertTrue(json.contains("\"operation_id\":\"playerBan\""));
        assertTrue(json.contains("\"deprecated\":true"));
        assertTrue(json.contains("\"input_schema\":\"{\\\"type\\\":\\\"object\\\"}\""));
    }

    @Test
    void getManifestGzippedReturnsValidGzip() throws IOException, CroupierException {
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        byte[] gzipped = client.getManifestGzipped();
        try (GZIPInputStream gzip = new GZIPInputStream(new ByteArrayInputStream(gzipped))) {
            String json = new String(gzip.readAllBytes(), StandardCharsets.UTF_8);
            assertTrue(json.contains("\"provider\":"));
        }
    }

    private static final class TestHarness {
        private final AtomicReference<SdkWireMessages.ProviderConnectRequest> registerRequest = new AtomicReference<>();
        private final AtomicReference<SdkWireMessages.InvokeRequest> lastRemoteInvoke = new AtomicReference<>();
        private final CountDownLatch heartbeatLatch = new CountDownLatch(1);
        private final AtomicInteger remoteStreamCount = new AtomicInteger();

        private FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    registerRequest.set(SdkWireMessages.decodeProviderConnectRequest(data));
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-1")
                    );
                }
                if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    heartbeatLatch.countDown();
                    return new byte[0];
                }
                if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                    lastRemoteInvoke.set(SdkWireMessages.decodeInvokeRequest(data));
                    remoteStreamCount.set(0);
                    return SdkWireMessages.encodeStartTaskResponse(
                        new SdkWireMessages.StartTaskResponse("remote-task-1")
                    );
                }
                if (msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                    int call = remoteStreamCount.getAndIncrement();
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
                            "{\"ok\":1}".getBytes(StandardCharsets.UTF_8)
                        )
                    );
                }
                if (msgType == Protocol.MSG_CANCEL_TASK_REQUEST || msgType == Protocol.MSG_INVOKE_REQUEST) {
                    return new byte[0];
                }
                throw new IllegalStateException("Unexpected message type " + msgType);
            });
        }
    }
}
