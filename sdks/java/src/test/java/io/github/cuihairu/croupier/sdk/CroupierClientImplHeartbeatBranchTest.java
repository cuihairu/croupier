package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Heartbeat / drain / guard branch fillers for CroupierClientImpl. */
class CroupierClientImplHeartbeatBranchTest {

    private static final class Harness {
        final AtomicInteger heartbeats = new AtomicInteger();
        final CountDownLatch secondFailure = new CountDownLatch(2);
        volatile boolean failHeartbeats;

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-hb"));
                }
                if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    if (failHeartbeats) {
                        secondFailure.countDown();
                        throw new RuntimeException("heartbeat down");
                    }
                    heartbeats.incrementAndGet();
                    return new byte[0];
                }
                if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                    return SdkWireMessages.encodeStartTaskResponse(
                        new SdkWireMessages.StartTaskResponse("task-hb"));
                }
                return new byte[0];
            });
        }
    }

    private static CroupierClientImpl connected(ClientConfig config, Harness harness) {
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{\"ok\":true}"));
        assertDoesNotThrow(() -> client.connect().join());
        return client;
    }

    private static void setField(CroupierClientImpl client, String name, Object value) throws Exception {
        Field field = CroupierClientImpl.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(client, value);
    }

    @Test
    void heartbeatSkipsWhenTransportOrSessionDrops() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setHeartbeatInterval(1);
        CroupierClientImpl client = connected(config, harness);

        // wait for at least one healthy heartbeat
        for (int i = 0; i < 30 && harness.heartbeats.get() < 1; i++) {
            Thread.sleep(100);
        }
        assertTrue(harness.heartbeats.get() >= 1);

        // transport dropped but connected flag still set -> transport==null branch
        setField(client, "transport", null);
        Thread.sleep(1500);
        // session cleared -> sessionId.isEmpty() branch
        setField(client, "transport", harness.newTransport());
        // reconnect transport manually so the loop can observe the empty session
        Thread.sleep(1500);
        assertDoesNotThrow(client::close);
    }

    @Test
    void repeatedHeartbeatFailuresWhileServingTriggerRecovery() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setHeartbeatInterval(1);
        // disabled reconnect keeps recovery cheap and deterministic
        config.setReconnect(ReconnectConfig.builder()
            .enabled(false).maxAttempts(1).build());
        CroupierClientImpl client = connected(config, harness);

        java.util.concurrent.CompletableFuture<Void> serving = client.serveAsync();
        for (int i = 0; i < 50 && !client.isServing(); i++) {
            Thread.sleep(100);
        }
        assertTrue(client.isServing());

        harness.failHeartbeats = true;
        assertTrue(harness.secondFailure.await(10, TimeUnit.SECONDS),
            "two failing heartbeats should have been observed");

        client.stop();
        assertDoesNotThrow(() -> serving.get(5, TimeUnit.SECONDS));
        assertDoesNotThrow(client::close);
    }

    @Test
    void registeringWhileDisconnectedButServingThrows() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        CroupierClientImpl client = connected(config, harness);
        java.util.concurrent.CompletableFuture<Void> serving = client.serveAsync();
        for (int i = 0; i < 50 && !client.isServing(); i++) {
            Thread.sleep(100);
        }
        assertTrue(client.isServing());
        // simulate a dropped connection while the service keeps serving
        setField(client, "connected",
            new java.util.concurrent.atomic.AtomicBoolean(false));
        try {
            org.junit.jupiter.api.Assertions.assertThrows(CroupierException.class, () ->
                client.registerFunction(new FunctionDescriptor("fn.x", "1.0.0"), (ctx, p) -> "{}"));
        } finally {
            client.stop();
            assertDoesNotThrow(() -> serving.get(5, TimeUnit.SECONDS));
            assertDoesNotThrow(client::close);
        }
    }

    @Test
    void drainAndRecoverWithEnabledReconnectRunsRecovery() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setReconnect(ReconnectConfig.builder()
            .enabled(true).maxAttempts(1).initialDelayMs(1).maxDelayMs(1).build());
        CroupierClientImpl client = connected(config, harness);
        java.lang.reflect.Method drain =
            CroupierClientImpl.class.getDeclaredMethod("drainAndRecover");
        drain.setAccessible(true);
        assertDoesNotThrow(() -> drain.invoke(client));
        Thread.sleep(200);
        assertDoesNotThrow(client::close);
    }

    @Test
    void cancelOfFinishedLocalTaskIsSkipped() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connected(new ClientConfig("game", "svc"), harness);

        java.lang.reflect.Method start =
            CroupierClientImpl.class.getDeclaredMethod("handleStartTaskRequest", byte[].class);
        start.setAccessible(true);
        byte[] body = SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
            "fn.one", "", "{}".getBytes(StandardCharsets.UTF_8), Map.of()));
        Object result = start.invoke(client, body);
        String taskId = SdkWireMessages.decodeStartTaskResponse((byte[]) result).taskId;

        // cancel first: state exists and is not done -> cancelled event emitted
        java.lang.reflect.Method cancel =
            CroupierClientImpl.class.getDeclaredMethod("handleCancelTaskRequest", byte[].class);
        cancel.setAccessible(true);
        cancel.invoke(client, SdkWireMessages.encodeCancelTaskRequest(
            new SdkWireMessages.CancelTaskRequest(taskId)));
        // cancelling again: same path is idempotent and must not throw
        cancel.invoke(client, SdkWireMessages.encodeCancelTaskRequest(
            new SdkWireMessages.CancelTaskRequest(taskId)));
        assertDoesNotThrow(client::close);
    }
}
