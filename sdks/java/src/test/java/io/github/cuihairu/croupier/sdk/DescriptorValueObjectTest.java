package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Coverage for small descriptor value objects and the CroupierSDK descriptor builder.
 */
@DisplayName("Descriptor value objects and builder")
class DescriptorValueObjectTest {

    @Test
    @DisplayName("FunctionDescriptor capability/execution/approval accessors round-trip")
    void functionDescriptorAccessors() {
        FunctionDescriptor descriptor = new FunctionDescriptor("f", "1.0.0");
        descriptor.setCapability("cap");
        assertEquals("cap", descriptor.getCapability());
        descriptor.setExecution("exec");
        assertEquals("exec", descriptor.getExecution());
        descriptor.setApprovalRequired(true);
        assertTrue(descriptor.isApprovalRequired());
        descriptor.setApprovalPolicyKey("policy");
        assertEquals("policy", descriptor.getApprovalPolicyKey());
        descriptor.setTags(List.of("t"));
        assertEquals(List.of("t"), descriptor.getTags());
    }

    @Test
    @DisplayName("ProviderFunctionDescriptor capability/execution accessors round-trip")
    void providerFunctionDescriptorAccessors() {
        ProviderFunctionDescriptor descriptor = new ProviderFunctionDescriptor("f", "1.0.0");
        assertNull(descriptor.getCapability());
        descriptor.setCapability("cap");
        assertEquals("cap", descriptor.getCapability());
        assertNull(descriptor.getExecution());
        descriptor.setExecution("exec");
        assertEquals("exec", descriptor.getExecution());
    }

    @Test
    @DisplayName("CroupierSDK descriptor builder sets capability and execution")
    void sdkDescriptorBuilder() {
        FunctionDescriptor descriptor = CroupierSDK.functionDescriptor("f", "2.0.0")
            .capability("cap-3")
            .execution("exec-3")
            .build();
        assertEquals("f", descriptor.getId());
        assertEquals("2.0.0", descriptor.getVersion());
        assertEquals("cap-3", descriptor.getCapability());
        assertEquals("exec-3", descriptor.getExecution());
    }
}
