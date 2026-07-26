package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Additional tests for CroupierClientImpl to improve code coverage.
 */
class CroupierClientImplCoverageTest {

    private ClientConfig createConfig() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        config.setServiceVersion("1.0.0");
        return config;
    }

    private FakeTransportClient createFakeTransport() {
        return new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-1")
                );
            }
            if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                return new byte[0];
            }
            return new byte[0];
        });
    }

    @Test
    @DisplayName("registerFunction with empty ID should throw")
    void registerFunctionEmptyId() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("", "1.0.0");
        assertThrows(CroupierException.class, () ->
            client.registerFunction(desc, (ctx, payload) -> "ok"));
    }

    @Test
    @DisplayName("registerFunction with empty version should throw")
    void registerFunctionEmptyVersion() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("func", "");
        assertThrows(CroupierException.class, () ->
            client.registerFunction(desc, (ctx, payload) -> "ok"));
    }

    @Test
    @DisplayName("connect should be idempotent")
    void connectIdempotent() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        client.connect().join();
        client.connect().join(); // Second call should be no-op

        assertTrue(client.isConnected());
    }

    @Test
    @DisplayName("serve should connect if not connected")
    void serveAutoConnect() throws Exception {
        AtomicBoolean serving = new AtomicBoolean(false);
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        // Start serve in background
        Thread serveThread = new Thread(() -> {
            try {
                client.serve();
            } catch (CroupierException e) {
                // Expected when stopped
            }
        });
        serveThread.start();

        // Wait a bit then stop
        Thread.sleep(500);
        client.stop();
        serveThread.join(2000);

        assertFalse(client.isServing());
    }

    @Test
    @DisplayName("stop should cleanup resources")
    void stopCleanup() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        client.stop();

        assertFalse(client.isConnected());
        assertFalse(client.isServing());
        assertEquals("", client.getSessionId());
    }

    @Test
    @DisplayName("close should cleanup all resources")
    void closeCleanup() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        client.close();

        assertFalse(client.isConnected());
    }

    @Test
    @DisplayName("isConnected should return false initially")
    void isConnectedInitiallyFalse() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        assertFalse(client.isConnected());
    }

    @Test
    @DisplayName("isServing should return false initially")
    void isServingInitiallyFalse() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        assertFalse(client.isServing());
    }

    @Test
    @DisplayName("getSessionId should return empty initially")
    void getSessionIdInitiallyEmpty() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        assertEquals("", client.getSessionId());
    }

    @Test
    @DisplayName("invoke should call registered handler")
    void invokeHandler() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"),
            (ctx, payload) -> "result:" + payload);

        String result = client.invoke("f1", "input", Map.of("key", "value"));

        assertEquals("result:input", result);
    }

    @Test
    @DisplayName("invoke should throw for unknown function")
    void invokeUnknownFunction() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        assertThrows(CroupierException.class, () ->
            client.invoke("unknown", "payload", Map.of()));
    }

    @Test
    @DisplayName("invoke should handle null metadata")
    void invokeNullMetadata() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"),
            (ctx, payload) -> "ok");

        String result = client.invoke("f1", "payload", null);

        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke should handle handler exception")
    void invokeHandlerException() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"),
            (ctx, payload) -> { throw new RuntimeException("handler error"); });

        CroupierException ex = assertThrows(CroupierException.class, () ->
            client.invoke("f1", "payload", Map.of()));

        assertTrue(ex.getMessage().contains("handler error"));
    }

    @Test
    @DisplayName("invoke should handle CroupierException from handler")
    void invokeHandlerCroupierException() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"),
            (ctx, payload) -> { throw new CroupierException("custom error"); });

        CroupierException ex = assertThrows(CroupierException.class, () ->
            client.invoke("f1", "payload", Map.of()));

        assertEquals("custom error", ex.getMessage());
    }

    @Test
    @DisplayName("buildManifest should include provider info")
    void buildManifestProviderInfo() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\"provider\":"));
        assertTrue(json.contains("\"id\":\"svc-1\""));
        assertTrue(json.contains("\"version\":\"1.0.0\""));
    }

    @Test
    @DisplayName("buildManifest should include functions")
    void buildManifestFunctions() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\"functions\":"));
        assertTrue(json.contains("\"id\":\"f1\""));
    }

    @Test
    @DisplayName("buildManifest with extended fields")
    void buildManifestExtendedFields() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setTags(java.util.List.of("tag1", "tag2"));
        desc.setSummary("Summary");
        desc.setDescription("Description");
        desc.setOperationId("opId");
        desc.setDeprecated(true);
        desc.setInputSchema("{\"type\":\"object\"}");
        desc.setOutputSchema("{\"type\":\"object\"}");
        desc.setResource("player");
        desc.setRisk("danger");
        desc.setOperation("ban");
        desc.setPermission("player.ban");
        desc.setEnabled(true);

        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\"tags\":[\"tag1\",\"tag2\"]"));
        assertTrue(json.contains("\"summary\":\"Summary\""));
        assertTrue(json.contains("\"description\":\"Description\""));
        assertTrue(json.contains("\"operation_id\":\"opId\""));
        assertTrue(json.contains("\"deprecated\":true"));
        assertTrue(json.contains("\"enabled\":true"));
        assertTrue(json.contains("\"resource\":\"player\""));
        assertTrue(json.contains("\"operation\":\"ban\""));
        assertTrue(json.contains("\"permission\":\"player.ban\""));
        assertFalse(json.contains("\"category\""));
        assertFalse(json.contains("\"entity\""));
        assertFalse(json.contains("\"placement\""));
        assertFalse(json.contains("\"page_hint\""));
    }

    @Test
    @DisplayName("getManifestGzipped should return valid gzip")
    void getManifestGzipped() throws CroupierException, IOException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        byte[] gzipped = client.getManifestGzipped();

        assertNotNull(gzipped);
        assertTrue(gzipped.length > 0);
    }

    @Test
    @DisplayName("getLocalFunctions should return registered functions")
    void getLocalFunctions() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.registerFunction(new FunctionDescriptor("f2", "2.0.0"), (ctx, payload) -> "ok");

        var functions = client.getLocalFunctions();

        assertEquals(2, functions.size());
    }

    @Test
    @DisplayName("getRegisterRequest should return valid map")
    void getRegisterRequest() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        FunctionDescriptor descriptor = CroupierSDK.functionDescriptor("f1", "1.0.0")
            .resource("player")
            .operation("ban")
            .risk("danger")
            .permission("player.ban")
            .build();
        client.registerFunction(descriptor, (ctx, payload) -> "ok");

        Map<String, Object> request = client.getRegisterRequest();

        assertEquals("svc-1", request.get("serviceId"));
        assertEquals("1.0.0", request.get("version"));
        @SuppressWarnings("unchecked")
        var functions = (java.util.List<Map<String, Object>>) request.get("functions");
        assertEquals(1, functions.size());
        assertEquals("player", functions.get(0).get("resource"));
        assertEquals("ban", functions.get(0).get("operation"));
        assertEquals("danger", functions.get(0).get("risk"));
        assertEquals("player.ban", functions.get(0).get("permission"));
        assertFalse(functions.get(0).containsKey("category_display"));
        assertFalse(functions.get(0).containsKey("entity_display"));
        assertFalse(functions.get(0).containsKey("operation_kind"));
        assertFalse(functions.get(0).containsKey("placement"));
        assertFalse(functions.get(0).containsKey("page_hint"));
        assertFalse(functions.get(0).containsKey("extensions"));
    }

    @Test
    @DisplayName("connect should fail with empty session ID")
    void connectEmptySessionId() throws CroupierException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("")
                );
            }
            return new byte[0];
        });

        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        assertThrows(RuntimeException.class, () -> client.connect().join());
    }

    @Test
    @DisplayName("startTask should delegate to invoker")
    void startTaskDelegation() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-1")
                );
            }
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-1")
                );
            }
            return new byte[0];
        });

        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        String taskId = client.startTask("f1", "{}", Map.of("key", "value"));

        assertEquals("task-1", taskId);
    }

    @Test
    @DisplayName("cancelTask should return true on success")
    void cancelTaskSuccess() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-1")
                );
            }
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-1")
                );
            }
            return new byte[0];
        });

        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        String taskId = client.startTask("f1", "{}");
        boolean result = client.cancelTask(taskId);

        assertTrue(result);
    }

    @Test
    @DisplayName("buildManifest with null tags should not include tags")
    void buildManifestNullTags() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setTags(null);

        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertFalse(json.contains("\"tags\":"));
    }

    @Test
    @DisplayName("buildManifest with empty tags should not include tags")
    void buildManifestEmptyTags() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setTags(java.util.List.of());

        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertFalse(json.contains("\"tags\":"));
    }

    @Test
    @DisplayName("buildManifest with special characters should escape properly")
    void buildManifestSpecialChars() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Summary with \"quotes\" and \\backslash");

        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\\"quotes\\\""));
        assertTrue(json.contains("\\\\backslash"));
    }

    @Test
    @DisplayName("validateConfig with unknown env should warn")
    void validateConfigUnknownEnv() {
        ClientConfig config = createConfig();
        config.setEnv("unknown");

        // Should not throw, just warn
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> createFakeTransport());

        assertNotNull(client);
    }

    @Test
    @DisplayName("validateConfig with empty gameId should warn")
    void validateConfigEmptyGameId() {
        ClientConfig config = createConfig();
        config.setGameId("");

        // Should not throw, just warn
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> createFakeTransport());

        assertNotNull(client);
    }

    @Test
    @DisplayName("connect with no handlers should fail")
    void connectNoHandlers() {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        assertThrows(RuntimeException.class, () -> client.connect().join());
    }

    @Test
    @DisplayName("registerFunction after connect should throw")
    void registerAfterConnect() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        assertThrows(CroupierException.class, () ->
            client.registerFunction(new FunctionDescriptor("f2", "1.0.0"), (ctx, payload) -> "ok"));
    }

    @Test
    @DisplayName("invoke with null metadata should use empty map")
    void invokeWithNullMetadata() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"),
            (ctx, payload) -> "result:" + payload);

        String result = client.invoke("f1", "input", null);
        assertEquals("result:input", result);
    }

    @Test
    @DisplayName("invoke with empty metadata should work")
    void invokeWithEmptyMetadata() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"),
            (ctx, payload) -> "ok");

        String result = client.invoke("f1", "payload", Map.of());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("startTask with metadata should delegate")
    void startTaskWithMetadata() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-1")
                );
            }
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-1")
                );
            }
            return new byte[0];
        });

        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        String taskId = client.startTask("f1", "{}", Map.of("key", "value"));
        assertEquals("task-1", taskId);
    }

    @Test
    @DisplayName("startTask with null metadata should use empty map")
    void startTaskWithNullMetadata() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-1")
                );
            }
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-1")
                );
            }
            return new byte[0];
        });

        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        String taskId = client.startTask("f1", "{}", null);
        assertEquals("task-1", taskId);
    }

    @Test
    @DisplayName("getRegisterRequest should include all functions")
    void getRegisterRequestMultipleFunctions() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.registerFunction(new FunctionDescriptor("f2", "2.0.0"), (ctx, payload) -> "ok");

        Map<String, Object> request = client.getRegisterRequest();

        assertEquals("svc-1", request.get("serviceId"));
        assertEquals("1.0.0", request.get("version"));
        assertNotNull(request.get("functions"));
    }

    @Test
    @DisplayName("isConnected should return true after connect")
    void isConnectedAfterConnect() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        assertTrue(client.isConnected());
    }

    @Test
    @DisplayName("getSessionId should return session after connect")
    void getSessionIdAfterConnect() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().join();

        assertEquals("session-1", client.getSessionId());
    }

    @Test
    @DisplayName("close should work when not connected")
    void closeWhenNotConnected() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        // Should not throw
        client.close();
        assertFalse(client.isConnected());
    }

    @Test
    @DisplayName("stop should work when not serving")
    void stopWhenNotServing() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        // Should not throw
        client.stop();
        assertFalse(client.isServing());
    }
}
