package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Second-wave branch fillers for CroupierClientImpl internal paths. */
class CroupierClientImplRecoveryBranchTest {

    private static final class Harness {
        final CountDownLatch capabilitiesLatch = new CountDownLatch(1);

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-recovery"));
                }
                if (msgType == Protocol.MSG_REGISTER_CAPABILITIES_REQ) {
                    capabilitiesLatch.countDown();
                    return new byte[0];
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

    private static Object invokePrivate(CroupierClientImpl client, String method) throws Exception {
        Method m = CroupierClientImpl.class.getDeclaredMethod(method);
        m.setAccessible(true);
        return m.invoke(client);
    }

    @Test
    void recoverConnectionWithNullReconnectConfigStaysDisconnected() throws Exception {
        Harness harness = new Harness();
        // default ClientConfig carries a null ReconnectConfig
        CroupierClientImpl client = connected(new ClientConfig("game", "svc"), harness);
        assertDoesNotThrow(() -> invokePrivate(client, "recoverConnection"));
        Thread.sleep(100);
        assertDoesNotThrow(client::close);
    }

    @Test
    void drainAndRecoverWithNullReconnectConfigClosesTransport() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connected(new ClientConfig("game", "svc"), harness);
        assertDoesNotThrow(() -> invokePrivate(client, "drainAndRecover"));
        Thread.sleep(100);
        assertDoesNotThrow(client::close);
    }

    @Test
    void blankControlAddressSkipsCapabilitiesUpload() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setControlAddr("   ");
        CroupierClientImpl client = connected(config, harness);
        // blank address returns immediately: no capabilities request is issued
        assertTrue(!harness.capabilitiesLatch.await(300, TimeUnit.MILLISECONDS));
        assertDoesNotThrow(client::close);
    }

    @Test
    void inboundValidationSkipsFunctionsWithoutSchema() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setValidateInputPayloads(true);
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor noSchema = new FunctionDescriptor("fn.noschema", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(noSchema, (ctx, payload) -> "{\"ok\":1}"));
        FunctionDescriptor blankSchema = new FunctionDescriptor("fn.blankschema", "1.0.0");
        blankSchema.setInputSchema("   ");
        assertDoesNotThrow(() -> client.registerFunction(blankSchema, (ctx, payload) -> "{\"ok\":2}"));
        assertDoesNotThrow(() -> client.connect().join());

        // payloads for schema-less functions pass validation untouched
        Method handle = CroupierClientImpl.class.getDeclaredMethod(
            "handleStartTaskRequest", byte[].class);
        handle.setAccessible(true);
        for (String fn : new String[] {"fn.noschema", "fn.blankschema"}) {
            byte[] body = SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
                fn, "", "{}".getBytes(StandardCharsets.UTF_8), Map.of()));
            assertDoesNotThrow(() -> handle.invoke(client, body));
        }
        assertDoesNotThrow(client::close);
    }
}
