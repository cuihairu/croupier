package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for CroupierClientImpl escapeJson and buildManifest edge cases.
 */
class CroupierClientImplEscapeTest {

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
            return new byte[0];
        });
    }

    @Test
    @DisplayName("buildManifest with newline in summary should escape")
    void buildManifestNewline() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Line1\nLine2");
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\n"));
    }

    @Test
    @DisplayName("buildManifest with tab in summary should escape")
    void buildManifestTab() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Col1\tCol2");
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\t"));
    }

    @Test
    @DisplayName("buildManifest with carriage return should escape")
    void buildManifestCarriageReturn() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Line1\rLine2");
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\r"));
    }

    @Test
    @DisplayName("buildManifest with backspace should escape")
    void buildManifestBackspace() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Text\bMore");
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\b"));
    }

    @Test
    @DisplayName("buildManifest with form feed should escape")
    void buildManifestFormFeed() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Text\fMore");
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\f"));
    }

    @Test
    @DisplayName("buildManifest with control character should escape as unicode")
    void buildManifestControlChar() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("TextMore");
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\\u0001"));
    }

    @Test
    @DisplayName("buildManifest with multiple functions should separate with comma")
    void buildManifestMultipleFunctions() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");
        client.registerFunction(new FunctionDescriptor("f2", "2.0.0"), (ctx, payload) -> "ok");
        client.registerFunction(new FunctionDescriptor("f3", "3.0.0"), (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\"id\":\"f1\""));
        assertTrue(json.contains("\"id\":\"f2\""));
        assertTrue(json.contains("\"id\":\"f3\""));
        // Should have comma separators between functions
        assertTrue(json.contains("},{"));
    }

    @Test
    @DisplayName("buildManifest with all optional fields set")
    void buildManifestAllOptionalFields() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        desc.setSummary("Summary");
        desc.setDescription("Description");
        desc.setOperationId("opId");
        desc.setInputSchema("{\"type\":\"object\"}");
        desc.setOutputSchema("{\"type\":\"string\"}");
        desc.setCategory("game");
        desc.setRisk("low");
        desc.setEntity("Player");
        desc.setOperation("create");
        desc.setEnabled(true);

        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertTrue(json.contains("\"summary\":\"Summary\""));
        assertTrue(json.contains("\"description\":\"Description\""));
        assertTrue(json.contains("\"operation_id\":\"opId\""));
        assertTrue(json.contains("\"input_schema\":"));
        assertTrue(json.contains("\"output_schema\":"));
        assertTrue(json.contains("\"category\":\"game\""));
        assertTrue(json.contains("\"risk\":\"low\""));
        assertTrue(json.contains("\"entity\":\"Player\""));
        assertTrue(json.contains("\"operation\":\"create\""));
        assertTrue(json.contains("\"enabled\":true"));
    }

    @Test
    @DisplayName("buildManifest with null summary should not include it")
    void buildManifestNullSummary() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        FunctionDescriptor desc = new FunctionDescriptor("f1", "1.0.0");
        // summary is null by default
        client.registerFunction(desc, (ctx, payload) -> "ok");

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertFalse(json.contains("\"summary\":"));
    }

    @Test
    @DisplayName("buildManifest with no functions should not include functions array")
    void buildManifestNoFunctions() throws CroupierException {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());

        byte[] manifest = client.buildManifest();
        String json = new String(manifest, StandardCharsets.UTF_8);

        assertFalse(json.contains("\"functions\":"));
    }

    @Test
    @DisplayName("getManifestGzipped should return valid gzip data")
    void getManifestGzippedValid() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(),
            (address, timeout) -> createFakeTransport());
        client.registerFunction(new FunctionDescriptor("f1", "1.0.0"), (ctx, payload) -> "ok");

        byte[] gzipped = client.getManifestGzipped();

        assertNotNull(gzipped);
        assertTrue(gzipped.length > 0);
        // Gzip magic bytes
        assertEquals(0x1f, gzipped[0] & 0xff);
        assertEquals(0x8b, gzipped[1] & 0xff);
    }
}
