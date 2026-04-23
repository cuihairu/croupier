package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class CroupierSDKTest {

    @Test
    void createClientWithConfig() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        CroupierClient client = CroupierSDK.createClient(config);

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    void createClientWithGameAndServiceIds() {
        CroupierClient client = CroupierSDK.createClient("game2", "svc2");

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    void createClientWithGameServiceAndAgentAddress() {
        CroupierClient client = CroupierSDK.createClient("game3", "svc3", "localhost:9999");

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    void functionDescriptorReturnsBuilder() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("test-func", "1.0.0");

        assertNotNull(builder);

        FunctionDescriptor desc = builder
            .category("test")
            .risk("low")
            .entity("player")
            .operation("create")
            .build();

        assertEquals("test-func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
        assertEquals("test", desc.getCategory());
        assertEquals("low", desc.getRisk());
        assertEquals("player", desc.getEntity());
        assertEquals("create", desc.getOperation());
        assertTrue(desc.isEnabled());
    }

    @Test
    void functionDescriptorBuilderWithDisabled() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", "1.0.0")
            .enabled(false)
            .build();

        assertFalse(desc.isEnabled());
    }

    @Test
    void functionDescriptorBuilderWithAllOptions() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.2.0")
            .category("player")
            .risk("high")
            .entity("player")
            .operation("update")
            .enabled(true)
            .build();

        assertEquals("player.ban", desc.getId());
        assertEquals("1.2.0", desc.getVersion());
        assertEquals("player", desc.getCategory());
        assertEquals("high", desc.getRisk());
        assertEquals("player", desc.getEntity());
        assertEquals("update", desc.getOperation());
        assertTrue(desc.isEnabled());
    }
}
