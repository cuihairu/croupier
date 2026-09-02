package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * F：Provider 侧入站 payload 校验（validateInboundPayload）。
 */
public class InboundValidationTest {

    private CroupierClientImpl newClient(boolean validate) {
        ClientConfig config = new ClientConfig();
        config.setValidateInputPayloads(validate);
        return new CroupierClientImpl(config);
    }

    private CroupierClientImpl clientWithSchema(boolean validate) throws CroupierException {
        CroupierClientImpl client = newClient(validate);
        FunctionDescriptor descriptor = new FunctionDescriptor("player.ban", "1.0.0");
        descriptor.setInputSchema(
                "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}},\"required\":[\"id\"]}");
        client.registerFunction(descriptor, (metadata, payload) -> "ok");
        return client;
    }

    private String validate(CroupierClientImpl client, String functionId, String payload)
            throws Exception {
        Method method = CroupierClientImpl.class.getDeclaredMethod(
                "validateInboundPayload", String.class, String.class);
        method.setAccessible(true);
        return (String) method.invoke(client, functionId, payload);
    }

    @Test
    public void disabledFlagSkipsValidation() throws Exception {
        CroupierClientImpl client = clientWithSchema(false);
        assertNull(validate(client, "player.ban", "{}"));
    }

    @Test
    public void missingRequiredRejected() throws Exception {
        CroupierClientImpl client = clientWithSchema(true);
        String error = validate(client, "player.ban", "{}");
        assertNotNull(error);
        assertTrue(error.contains("payload validation failed"));
    }

    @Test
    public void typeMismatchRejected() throws Exception {
        CroupierClientImpl client = clientWithSchema(true);
        String error = validate(client, "player.ban", "{\"id\":123}");
        assertNotNull(error);
        assertTrue(error.contains("payload validation failed"));
    }

    @Test
    public void validPayloadPasses() throws Exception {
        CroupierClientImpl client = clientWithSchema(true);
        assertNull(validate(client, "player.ban", "{\"id\":\"p1\"}"));
    }

    @Test
    public void unknownFunctionSkipsValidation() throws Exception {
        CroupierClientImpl client = newClient(true);
        assertNull(validate(client, "ghost", "{}"));
    }

    @Test
    public void emptySchemaSkipsValidation() throws Exception {
        CroupierClientImpl client = newClient(true);
        client.registerFunction(new FunctionDescriptor("player.free", "1.0.0"),
                (metadata, payload) -> "ok");
        assertNull(validate(client, "player.free", "{}"));
    }
}
