package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Fourth-wave fillers: heartbeat connected-flag branch and poller exit path. */
class CroupierClientImplFlagBranchTest {

    private static final class Harness {
        final AtomicInteger heartbeats = new AtomicInteger();

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-flag"));
                }
                if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    heartbeats.incrementAndGet();
                    return new byte[0];
                }
                if (msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                    return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent(
                        "progress", "working", 10, new byte[0]));
                }
                return new byte[0];
            });
        }
    }

    @Test
    void heartbeatSkipsWhenConnectedFlagDrops() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setHeartbeatInterval(1);
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{}"));
        assertDoesNotThrow(() -> client.connect().join());

        int before = harness.heartbeats.get();
        Field connectedField = CroupierClientImpl.class.getDeclaredField("connected");
        connectedField.setAccessible(true);
        connectedField.set(client, new java.util.concurrent.atomic.AtomicBoolean(false));
        Thread.sleep(1600);
        int after = harness.heartbeats.get();
        assertTrue(after <= before + 1, "heartbeat must be suppressed while disconnected");
        assertDoesNotThrow(client::close);
    }

    @Test
    void heartbeatStopsCleanlyOnCloseRace() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setHeartbeatInterval(1);
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{}"));
        assertDoesNotThrow(() -> client.connect().join());
        for (int i = 0; i < 30 && harness.heartbeats.get() < 1; i++) {
            Thread.sleep(100);
        }
        assertTrue(harness.heartbeats.get() >= 1);
        // close right after a heartbeat tick: the loop wakes and observes stop
        assertDoesNotThrow(client::close);
        Thread.sleep(1200);
        int afterClose = harness.heartbeats.get();
        Thread.sleep(1200);
        assertTrue(harness.heartbeats.get() <= afterClose + 1,
            "heartbeat loop must stop after close");
    }
}
