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
        desc.setCategory("test");
        desc.setRisk("low");
        desc.setEntity("player");
        desc.setOperation("read");
        desc.setEnabled(false);

        assertEquals("new.func", desc.getId());
        assertEquals("2.0.0", desc.getVersion());
        assertEquals("test", desc.getCategory());
        assertEquals("low", desc.getRisk());
        assertEquals("player", desc.getEntity());
        assertEquals("read", desc.getOperation());
        assertFalse(desc.isEnabled());
    }

    @Test
    void toStringContainsAllFields() {
        FunctionDescriptor desc = new FunctionDescriptor("test.func", "1.0.0");
        desc.setCategory("test");
        desc.setRisk("medium");

        String str = desc.toString();
        assertTrue(str.contains("test.func"));
        assertTrue(str.contains("1.0.0"));
        assertTrue(str.contains("test"));
        assertTrue(str.contains("medium"));
    }

    @Test
    void croupierSDKBuilderMethod() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("test.func", "1.0.0");

        assertNotNull(builder);
        FunctionDescriptor desc = builder
            .category("test")
            .risk("low")
            .entity("player")
            .operation("create")
            .enabled(true)
            .build();

        assertEquals("test.func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
        assertEquals("test", desc.getCategory());
        assertEquals("low", desc.getRisk());
        assertEquals("player", desc.getEntity());
        assertEquals("create", desc.getOperation());
        assertTrue(desc.isEnabled());
    }
}
