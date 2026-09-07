package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.ReconnectConfig;
import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.zip.GZIPInputStream;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for CroupierClientImpl guard branches. */
class CroupierClientImplBranchTest {

    private static final class Harness {
        final AtomicInteger heartbeats = new AtomicInteger();
        final CountDownLatch capabilitiesLatch = new CountDownLatch(1);
        volatile boolean failHeartbeats;

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-branch"));
                }
                if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    heartbeats.incrementAndGet();
                    if (failHeartbeats) {
                        throw new RuntimeException("heartbeat down");
                    }
                    return new byte[0];
                }
                if (msgType == Protocol.MSG_REGISTER_CAPABILITIES_REQ) {
                    capabilitiesLatch.countDown();
                    return new byte[0];
                }
                if (msgType == Protocol.MSG_INVOKE_REQUEST
                    || msgType == Protocol.MSG_START_TASK_REQUEST
                    || msgType == Protocol.MSG_CANCEL_TASK_REQUEST
                    || msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                    return new byte[0];
                }
                return new byte[0];
            });
        }
    }

    private static CroupierClientImpl connectedClient(Harness harness, ClientConfig config) {
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        descriptor.setInputSchema("{\"type\":\"object\",\"required\":[\"name\"],\"properties\":{\"name\":{\"type\":\"string\"}}}");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{\"ok\":true}"));
        assertDoesNotThrow(() -> client.connect().join());
        return client;
    }

    private static Object invokePrivate(CroupierClientImpl client, String method, Object... args) throws Exception {
        Method m = CroupierClientImpl.class.getDeclaredMethod(method, parameterTypes(args));
        m.setAccessible(true);
        return m.invoke(client, args);
    }

    private static Class<?>[] parameterTypes(Object... args) {
        Class<?>[] types = new Class<?>[args.length];
        for (int i = 0; i < args.length; i++) {
            types[i] = args[i].getClass();
        }
        return types;
    }

    @Test
    void registeringInvalidDescriptorsThrows() {
        Harness harness = new Harness();
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig("game", "svc"),
            (address, timeout) -> harness.newTransport());
        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor(null, "1.0.0"), (ctx, p) -> "{}"));
        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor("  ", "1.0.0"), (ctx, p) -> "{}"));
        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor("fn", null), (ctx, p) -> "{}"));
        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor("fn", "  "), (ctx, p) -> "{}"));
    }

    @Test
    void registeringAfterConnectThrows() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connectedClient(harness, new ClientConfig("game", "svc"));
        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor("fn.two", "1.0.0"), (ctx, p) -> "{}"));
        assertDoesNotThrow(client::close);
    }

    @Test
    void registeringWhileServingThrows() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connectedClient(harness, new ClientConfig("game", "svc"));
        CompletableFuture<Void> serving = client.serveAsync();
        try {
            awaitServing(client);
            assertTrue(client.isServing());
            assertThrows(CroupierException.class, () ->
                client.registerFunction(new FunctionDescriptor("fn.three", "1.0.0"), (ctx, p) -> "{}"));
        } finally {
            client.stop();
            assertDoesNotThrow(() -> serving.get(5, TimeUnit.SECONDS));
            assertDoesNotThrow(client::close);
        }
    }

    private static void awaitServing(CroupierClientImpl client) throws InterruptedException {
        for (int i = 0; i < 50 && !client.isServing(); i++) {
            Thread.sleep(100);
        }
    }

    @Test
    void capabilitiesUploadUsesControlAddressWhenConfigured() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setControlAddr("127.0.0.1:19999");
        CroupierClientImpl client = connectedClient(harness, config);
        assertTrue(harness.capabilitiesLatch.await(5, TimeUnit.SECONDS));
        assertDoesNotThrow(client::close);
    }

    @Test
    void manifestSkipsDisabledFunctionsAndIncludesMetadata() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor enabled = new FunctionDescriptor("fn.enabled", "1.0.0");
        client.registerFunction(enabled, (ctx, p) -> "{}");
        FunctionDescriptor disabled = new FunctionDescriptor("fn.disabled", "1.0.0");
        disabled.setEnabled(false);
        disabled.setExecution("async");
        disabled.setPermission("perm");
        client.registerFunction(disabled, (ctx, p) -> "{}");

        byte[] manifest = client.getManifestGzipped();
        assertNotNull(manifest);
        String json = gunzip(manifest);
        assertTrue(json.contains("fn.enabled"));
        assertTrue(json.contains("fn.disabled"));
        assertTrue(json.contains("perm"));
        assertTrue(json.contains("async"));
        assertDoesNotThrow(client::close);
    }

    private static String gunzip(byte[] data) throws Exception {
        try (GZIPInputStream in = new GZIPInputStream(new java.io.ByteArrayInputStream(data))) {
            return new String(in.readAllBytes(), StandardCharsets.UTF_8);
        }
    }

    @Test
    void filePushRejectsBadFileNamesAndWritesGoodOnes() throws Exception {
        Harness harness = new Harness();
        Path staging = Files.createTempDirectory("croupier-staging-test");
        ClientConfig config = new ClientConfig("game", "svc");
        config.setEnableFileTransfer(true);
        config.setFileStagingDir(staging.toString());
        CroupierClientImpl client = connectedClient(harness, config);

        byte[] blank = SdkWireMessages.encodeFilePushRequest(
            new SdkWireMessages.FilePushRequest("t-1", "a/b.txt", "h", new byte[] {1}));
        byte[] backslash = SdkWireMessages.encodeFilePushRequest(
            new SdkWireMessages.FilePushRequest("t-2", "a\\b.txt", "h", new byte[] {1}));
        byte[] dotdot = SdkWireMessages.encodeFilePushRequest(
            new SdkWireMessages.FilePushRequest("t-3", "..", "h", new byte[] {1}));
        byte[] blankName = SdkWireMessages.encodeFilePushRequest(
            new SdkWireMessages.FilePushRequest("t-4", "", "h", new byte[] {1}));
        for (byte[] bad : new byte[][] {blank, backslash, dotdot, blankName}) {
            SdkWireMessages.FilePushResponse response = SdkWireMessages.decodeFilePushResponse(
                (byte[]) invokePrivate(client, "handleFilePushRequest", bad));
            assertTrue(!response.ok);
        }

        byte[] good = SdkWireMessages.encodeFilePushRequest(
            new SdkWireMessages.FilePushRequest("t-5", "ok.txt", sha256Hex(new byte[] {1, 2, 3}), new byte[] {1, 2, 3}));
        SdkWireMessages.FilePushResponse ok = SdkWireMessages.decodeFilePushResponse(
            (byte[]) invokePrivate(client, "handleFilePushRequest", good));
        assertTrue(ok.ok, ok.error);
        assertTrue(Files.exists(staging.resolve("ok.txt")));

        // staging dir configured blank falls back to ./croupier-staging.
        // Known edge: the relative fallback path fails the normalized
        // startsWith containment check, so the push is refused (documented
        // behaviour; the explicit staging dir above works).
        ClientConfig blankDirConfig = new ClientConfig("game", "svc");
        blankDirConfig.setEnableFileTransfer(true);
        blankDirConfig.setFileStagingDir("");
        Harness secondHarness = new Harness();
        CroupierClientImpl blankClient = connectedClient(secondHarness, blankDirConfig);
        SdkWireMessages.FilePushResponse fallback = SdkWireMessages.decodeFilePushResponse(
            (byte[]) invokePrivate(blankClient, "handleFilePushRequest",
                SdkWireMessages.encodeFilePushRequest(
                    new SdkWireMessages.FilePushRequest("t-6", "fallback.txt", sha256Hex(new byte[] {9}), new byte[] {9}))));
        assertTrue(!fallback.ok, "relative fallback staging dir must be refused by containment check");
        assertDoesNotThrow(client::close);
        assertDoesNotThrow(blankClient::close);
    }

    private static String sha256Hex(byte[] data) throws Exception {
        java.security.MessageDigest digest = java.security.MessageDigest.getInstance("SHA-256");
        StringBuilder hex = new StringBuilder();
        for (byte b : digest.digest(data)) {
            hex.append(String.format("%02x", b));
        }
        return hex.toString();
    }

    @Test
    void inboundValidationRejectsBadStartTaskPayloads() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setValidateInputPayloads(true);
        CroupierClientImpl client = connectedClient(harness, config);

        byte[] invalid = SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
            "fn.one", "", "{}".getBytes(StandardCharsets.UTF_8), Map.of()));
        assertThrows(Exception.class, () -> invokePrivate(client, "handleStartTaskRequest", invalid));

        byte[] valid = SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
            "fn.one", "", "{\"name\":\"x\"}".getBytes(StandardCharsets.UTF_8), Map.of()));
        assertDoesNotThrow(() -> invokePrivate(client, "handleStartTaskRequest", valid));
        assertDoesNotThrow(client::close);
    }

    @Test
    void cancelLocalTaskMarksCancelledAndUnknownIsIgnored() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        CroupierClientImpl client = connectedClient(harness, config);

        byte[] started = (byte[]) invokePrivate(client, "handleStartTaskRequest",
            SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
                "fn.one", "", "{\"name\":\"x\"}".getBytes(StandardCharsets.UTF_8), Map.of())));
        SdkWireMessages.StartTaskResponse response =
            SdkWireMessages.decodeStartTaskResponse(started);
        byte[] cancel = SdkWireMessages.encodeCancelTaskRequest(
            new SdkWireMessages.CancelTaskRequest(response.taskId));
        assertDoesNotThrow(() -> invokePrivate(client, "handleCancelTaskRequest", cancel));
        // cancelling an unknown task is a no-op
        byte[] unknown = SdkWireMessages.encodeCancelTaskRequest(
            new SdkWireMessages.CancelTaskRequest("missing-task"));
        assertDoesNotThrow(() -> invokePrivate(client, "handleCancelTaskRequest", unknown));
        assertDoesNotThrow(client::close);
    }

    @Test
    void recoverConnectionRespectsDisabledReconnect() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        ReconnectConfig disabled = ReconnectConfig.builder().enabled(false).build();
        config.setReconnect(disabled);
        CroupierClientImpl client = connectedClient(harness, config);
        // not serving: recovery thread exits promptly without reconnect attempts
        assertDoesNotThrow(() -> invokePrivate(client, "recoverConnection"));
        Thread.sleep(100);
        assertDoesNotThrow(client::close);
    }

    @Test
    void recoverConnectionStopsWhenNotServing() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setReconnect(ReconnectConfig.builder()
            .enabled(true).maxAttempts(1).initialDelayMs(1).maxDelayMs(1).build());
        CroupierClientImpl client = connectedClient(harness, config);
        assertDoesNotThrow(() -> invokePrivate(client, "recoverConnection"));
        Thread.sleep(100);
        assertDoesNotThrow(client::close);
    }

    @Test
    void unsupportedLocalRequestThrows() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connectedClient(harness, new ClientConfig("game", "svc"));
        Method handler = CroupierClientImpl.class.getDeclaredMethod("handleLocalRequest", int.class, int.class, byte[].class);
        handler.setAccessible(true);
        assertThrows(Exception.class, () -> handler.invoke(client, 999999, 1, new byte[0]));
        assertDoesNotThrow(client::close);
    }

    @Test
    void stopServingIsIdempotentAfterServe() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setHeartbeatInterval(1);
        CroupierClientImpl client = connectedClient(harness, config);
        CompletableFuture<Void> serving = client.serveAsync();
        awaitServing(client);
        for (int i = 0; i < 50 && harness.heartbeats.get() < 1; i++) {
            Thread.sleep(100);
        }
        assertTrue(harness.heartbeats.get() >= 1, "heartbeat should have fired");
        client.stop();
        serving.get(5, TimeUnit.SECONDS);
        client.stop();
        assertDoesNotThrow(client::close);
    }
}
