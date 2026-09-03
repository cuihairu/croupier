package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * F：入站校验的 invokeInbound 回包路径——违规 payload 回
 * {"error":"payload validation failed: …"}，handler 不被调用。
 */
public class InboundInvokeResponseTest {

    private CroupierClientImpl newClient(boolean validate, FunctionDescriptor descriptor,
                                          FunctionHandler handler) throws CroupierException {
        ClientConfig config = new ClientConfig();
        config.setValidateInputPayloads(validate);
        CroupierClientImpl client = new CroupierClientImpl(config);
        if (descriptor != null) {
            client.registerFunction(descriptor, handler);
        }
        return client;
    }

    private byte[] handleInvoke(CroupierClientImpl client, String functionId, String payload)
            throws Exception {
        Method method = CroupierClientImpl.class.getDeclaredMethod("handleInvokeRequest", byte[].class);
        method.setAccessible(true);
        return (byte[]) method.invoke(client, SdkWireMessages.encodeInvokeRequest(
                new SdkWireMessages.InvokeRequest(functionId, "",
                        payload.getBytes(StandardCharsets.UTF_8), Map.of())));
    }

    private FunctionDescriptor descriptorWithRequiredId() {
        FunctionDescriptor descriptor = new FunctionDescriptor("player.ban", "1.0.0");
        descriptor.setInputSchema(
                "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}},\"required\":[\"id\"]}");
        return descriptor;
    }

    @Test
    @Timeout(5)
    public void invalidPayloadReturnsErrorAndSkipsHandler() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        CroupierClientImpl client = newClient(true, descriptorWithRequiredId(),
                (metadata, payload) -> {
                    calls.incrementAndGet();
                    return "ok";
                });

        byte[] response = handleInvoke(client, "player.ban", "{}");
        String body = new String(response, StandardCharsets.UTF_8);
        assertTrue(body.contains("payload validation failed"), "body=" + body);
        assertFalse(body.contains("ok"));
        assertEquals(0, calls.get(), "handler must not be invoked");
    }

    @Test
    @Timeout(5)
    public void validPayloadInvokesHandler() throws Exception {
        CroupierClientImpl client = newClient(true, descriptorWithRequiredId(),
                (metadata, payload) -> "done");
        byte[] response = handleInvoke(client, "player.ban", "{\"id\":\"p1\"}");
        assertEquals("done", new String(SdkWireMessages.decodeInvokeResponse(response).payload, StandardCharsets.UTF_8));
    }

    @Test
    @Timeout(5)
    public void disabledFlagKeepsLegacyBehavior() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        CroupierClientImpl client = newClient(false, descriptorWithRequiredId(),
                (metadata, payload) -> {
                    calls.incrementAndGet();
                    return "ok";
                });
        byte[] response = handleInvoke(client, "player.ban", "{}");
        assertEquals("ok", new String(SdkWireMessages.decodeInvokeResponse(response).payload, StandardCharsets.UTF_8));
        assertEquals(1, calls.get());
    }
}
