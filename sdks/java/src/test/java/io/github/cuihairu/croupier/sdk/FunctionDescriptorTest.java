package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class FunctionDescriptorTest {

    @Test
    void defaultConstructorCreatesEmptyDescriptor() {
        FunctionDescriptor desc = new FunctionDescriptor();

        assertNotNull(desc);
        assertTrue(desc.isEnabled());
    }

    @Test
    void constructorWithIdAndVersion() {
        FunctionDescriptor desc = new FunctionDescriptor("test.func", "1.0.0");

        assertEquals("test.func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
        assertTrue(desc.isEnabled());
    }

    @Test
    void settersAndGettersWork() {
        FunctionDescriptor desc = new FunctionDescriptor();
        desc.setId("new.func");
        desc.setVersion("2.0.0");
        desc.setResource("player");
        desc.setRisk("low");
        desc.setOperation("ban");
        desc.setPermission("player.ban");
        desc.setEnabled(false);

        assertEquals("new.func", desc.getId());
        assertEquals("2.0.0", desc.getVersion());
        assertEquals("player", desc.getResource());
        assertEquals("low", desc.getRisk());
        assertEquals("ban", desc.getOperation());
        assertEquals("player.ban", desc.getPermission());
        assertFalse(desc.isEnabled());
    }

    @Test
    void toStringContainsAllFields() {
        FunctionDescriptor desc = new FunctionDescriptor("test.func", "1.0.0");
        desc.setResource("player");
        desc.setRisk("medium");

        String str = desc.toString();
        assertTrue(str.contains("test.func"));
        assertTrue(str.contains("1.0.0"));
        assertTrue(str.contains("player"));
        assertTrue(str.contains("medium"));
    }

    @Test
    void croupierSDKBuilderMethod() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("test.func", "1.0.0");

        assertNotNull(builder);
        FunctionDescriptor desc = builder
            .resource("player")
            .risk("low")
            .operation("ban")
            .permission("player.ban")
            .enabled(true)
            .build();

        assertEquals("test.func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
        assertEquals("player", desc.getResource());
        assertEquals("low", desc.getRisk());
        assertEquals("ban", desc.getOperation());
        assertEquals("player.ban", desc.getPermission());
        assertTrue(desc.isEnabled());
    }
}
