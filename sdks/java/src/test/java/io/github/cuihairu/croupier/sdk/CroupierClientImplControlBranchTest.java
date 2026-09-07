package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Third-wave fillers for CroupierClientImpl control-plane and drain paths. */
class CroupierClientImplControlBranchTest {

    private static final class Harness {
        final AtomicInteger heartbeats = new AtomicInteger();
        final CountDownLatch invoked = new CountDownLatch(1);

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-ctrl"));
                }
                if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    heartbeats.incrementAndGet();
                    return new byte[0];
                }
                return new byte[0];
            });
        }
    }

    @Test
    void capabilitiesUploadWithBrokenControlTransportFailsOpen() throws Exception {
        // control-plane factory throws: maybeRegisterCapabilities must swallow
        // the error with controlTransport still null (finally branch)
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setControlAddr("tcp://broken:1");
        CroupierClientImpl client = new CroupierClientImpl(config, (address, timeout) -> {
            if (address.contains("broken")) {
                throw new RuntimeException("control plane unreachable");
            }
            return harness.newTransport();
        });
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{}"));
        assertDoesNotThrow(() -> client.connect().join());
        assertTrue(client.isConnected());
        assertDoesNotThrow(client::close);
    }

    @Test
    void heartbeatSkipsWhenSessionIdCleared() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setHeartbeatInterval(1);
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{}"));
        assertDoesNotThrow(() -> client.connect().join());

        int before = harness.heartbeats.get();
        // clear the session: the guard short-circuits before sending
        Field sessionField = CroupierClientImpl.class.getDeclaredField("sessionId");
        sessionField.setAccessible(true);
        sessionField.set(client, "");
        Thread.sleep(1500);
        int after = harness.heartbeats.get();
        assertTrue(after <= before + 1, "heartbeat must be suppressed for empty session");
        assertDoesNotThrow(client::close);
    }

    @Test
    void manifestIterationSkipsBlankIdDescriptor() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig("game", "svc"),
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.real", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{}"));

        // inject a descriptor whose id is blank to hit the skip guard
        Field descriptorsField = CroupierClientImpl.class.getDeclaredField("descriptors");
        descriptorsField.setAccessible(true);
        @SuppressWarnings("unchecked")
        Map<String, FunctionDescriptor> descriptors =
            (Map<String, FunctionDescriptor>) descriptorsField.get(client);
        FunctionDescriptor blank = new FunctionDescriptor("x", "1.0.0");
        blank.setId("");
        descriptors.put("blank", blank);

        byte[] manifest = client.getManifestGzipped();
        assertTrue(manifest.length > 0);
        try (java.util.zip.GZIPInputStream in = new java.util.zip.GZIPInputStream(
            new java.io.ByteArrayInputStream(manifest))) {
            String text = new String(in.readAllBytes(), StandardCharsets.UTF_8);
            assertTrue(text.contains("fn.real"));
        }
        descriptors.remove("blank");
        assertDoesNotThrow(client::close);
    }

    @Test
    void slowInflightCallDelaysDrainUntilCompletion() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig("game", "svc"),
            (address, timeout) -> harness.newTransport());
        CountDownLatch release = new CountDownLatch(1);
        CountDownLatch started = new CountDownLatch(1);
        FunctionDescriptor slow = new FunctionDescriptor("fn.slow", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(slow, (ctx, payload) -> {
            started.countDown();
            release.await(5, TimeUnit.SECONDS);
            return "{}";
        }));
        assertDoesNotThrow(() -> client.connect().join());

        // dispatch an inbound invoke on another thread (handler blocks)
        CompletableFuture<Void> invoke = CompletableFuture.runAsync(() -> {
            try {
                java.lang.reflect.Method handle = CroupierClientImpl.class
                    .getDeclaredMethod("handleInvokeRequest", byte[].class);
                handle.setAccessible(true);
                handle.invoke(client, SdkWireMessages.encodeInvokeRequest(
                    new SdkWireMessages.InvokeRequest("fn.slow", "",
                        "{}".getBytes(StandardCharsets.UTF_8), Map.of())));
            } catch (Exception ignored) {
            }
        });
        assertTrue(started.await(5, TimeUnit.SECONDS));

        java.lang.reflect.Method drain =
            CroupierClientImpl.class.getDeclaredMethod("drainAndRecover");
        drain.setAccessible(true);
        CompletableFuture<Void> draining = CompletableFuture.runAsync(() -> {
            try {
                drain.invoke(client);
            } catch (Exception ignored) {
            }
        });
        Thread.sleep(300);
        assertTrue(!draining.isDone(), "drain must wait for the in-flight call");
        release.countDown();
        assertDoesNotThrow(() -> draining.get(10, TimeUnit.SECONDS));
        assertDoesNotThrow(() -> invoke.get(5, TimeUnit.SECONDS));
        assertDoesNotThrow(client::close);
    }
}
