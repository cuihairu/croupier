package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Final branch fillers for the sdk root package: FieldHints normalize key
 * edges, OpenAPIImporter schema/extension edges and CroupierClientImpl
 * drain/reconnect/manifest guards.
 */
class BranchFillFinalTest {

    private static Object field(Object target, String name) throws Exception {
        Field f = target.getClass().getDeclaredField(name);
        f.setAccessible(true);
        return f.get(target);
    }

    // ------------------------------------------------------------------
    // FieldHints.normalizeHintKey: 'x' followed by neither '-' nor '_'
    // ------------------------------------------------------------------

    @Test
    void xPrefixedKeyWithoutSeparatorIsRejected() {
        FunctionDescriptor descriptor = new FunctionDescriptor("fn", "1.0.0");
        descriptor.setInputSchema("{}");
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor, "player", "xzwrong", "v"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor, "player", "x9", "v"));
    }

    // ------------------------------------------------------------------
    // OpenAPIImporter: non-map schema property + empty/absent x-resource
    // ------------------------------------------------------------------

    @Test
    void openapiNonMapPropertyAndEmptyResourceAreTolerated() {
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig("game-1", "svc-1"));
        String spec = """
            {
              "openapi": "3.0.0",
              "info": {"title": "t", "version": "1.0.0"},
              "paths": {
                "/a": {"post": {
                  "operationId": "opA",
                  "requestBody": {"content": {"application/json": {"schema": {
                    "type": "object",
                    "properties": {"amount": "number-should-be-an-object"}
                  }}}},
                  "x-resource": ""
                }},
                "/b": {"get": {"operationId": "opB"}}
              }
            }
            """;
        // The importer derives no version from the spec, so the strict client
        // rejects registration; continueOnError keeps the walk going (the
        // schema/extension branches run before registration anyway).
        var options = new OpenAPIImporter.ImportOptions().continueOnError(true);
        assertDoesNotThrow(() ->
            OpenAPIImporter.registerFromOpenAPI(client, spec, options, id -> (ctx, p) -> "{}"));
    }

    // ------------------------------------------------------------------
    // CroupierClientImpl.buildManifest: empty-id descriptor is skipped
    // ------------------------------------------------------------------

    @Test
    @SuppressWarnings("unchecked")
    void buildManifestSkipsEmptyIdDescriptor() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig("game-1", "svc-1"));
        Field descriptorsField = CroupierClientImpl.class.getDeclaredField("descriptors");
        descriptorsField.setAccessible(true);
        Map<String, FunctionDescriptor> descriptors =
            (Map<String, FunctionDescriptor>) descriptorsField.get(client);
        descriptors.put("", new FunctionDescriptor("", "1.0.0"));
        descriptors.put("real.fn", new FunctionDescriptor("real.fn", "1.0.0"));

        byte[] manifest = assertDoesNotThrow(client::buildManifest);
        assertNotNull(manifest);
        String json = new String(manifest, java.nio.charset.StandardCharsets.UTF_8);
        assertTrue(json.contains("real.fn"));
    }

    // ------------------------------------------------------------------
    // drainAndRecover / recoverConnection with null / disabled reconnect
    // ------------------------------------------------------------------

    private static byte[] drainAck(CroupierClientImpl client) throws Exception {
        byte[] body = SdkWireMessages.encodeProviderDrainRequest(
            new SdkWireMessages.ProviderDrainRequest("s-1", "rolling-restart", 1000));
        Method method = client.getClass().getDeclaredMethod("handleDrainRequest", byte[].class);
        method.setAccessible(true);
        return (byte[]) method.invoke(client, body);
    }

    private static void awaitFlag(CroupierClientImpl client, String name, boolean expected,
                                  long timeoutMs) throws Exception {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (((AtomicBoolean) field(client, name)).get() != expected
            && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
    }

    @Test
    void drainWithNullReconnectConfigLogsAndStaysDown() throws Exception {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setReconnect(null);
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), drainAck(client));
        // drainAndRecover: getReconnect() == null -> recoverConnection() ->
        // recovery thread warns (rc == null) and returns without retrying.
        awaitFlag(client, "draining", false, 5000);
        awaitFlag(client, "reconnecting", false, 5000);
        Thread.sleep(200);
        assertTrue(!((AtomicBoolean) field(client, "draining")).get());
    }

    @Test
    void recoverConnectionOnNonServingClientSkipsRetryLoop() throws Exception {
        // Default (enabled) reconnect config, but serving == false: the
        // while-loop condition must evaluate false and exit immediately.
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig("game-1", "svc-1"));
        Method recover = client.getClass().getDeclaredMethod("recoverConnection");
        recover.setAccessible(true);
        recover.invoke(client);
        awaitFlag(client, "reconnecting", false, 5000);
        assertTrue(!((AtomicBoolean) field(client, "reconnecting")).get());
    }
}
