package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class ProviderFunctionDescriptorTest {

    @Test
    void defaultConstructorCreatesEmptyDescriptor() {
        ProviderFunctionDescriptor desc = new ProviderFunctionDescriptor();

        assertNotNull(desc);
    }

    @Test
    void constructorWithIdAndVersion() {
        ProviderFunctionDescriptor desc = new ProviderFunctionDescriptor("local-func", "1.0.0");

        assertEquals("local-func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
    }

    @Test
    void settersWork() {
        ProviderFunctionDescriptor desc = new ProviderFunctionDescriptor("f", "1.0");

        desc.setId("updated-f");
        assertEquals("updated-f", desc.getId());

        desc.setVersion("3.0.0");
        assertEquals("3.0.0", desc.getVersion());
    }

    @Test
    void toStringContainsIdAndVersion() {
        ProviderFunctionDescriptor desc = new ProviderFunctionDescriptor("test-local", "1.0.0");
        String str = desc.toString();

        assertTrue(str.contains("test-local"));
        assertTrue(str.contains("1.0.0"));
    }

    @Test
    void equalsReturnsTrueForSameValues() {
        ProviderFunctionDescriptor desc1 = new ProviderFunctionDescriptor("func", "1.0.0");
        ProviderFunctionDescriptor desc2 = new ProviderFunctionDescriptor("func", "1.0.0");

        assertEquals(desc1, desc2);
        assertEquals(desc1.hashCode(), desc2.hashCode());
    }

    @Test
    void equalsReturnsFalseForDifferentValues() {
        ProviderFunctionDescriptor desc1 = new ProviderFunctionDescriptor("func1", "1.0.0");
        ProviderFunctionDescriptor desc2 = new ProviderFunctionDescriptor("func2", "1.0.0");

        assertNotEquals(desc1, desc2);
    }

    @Test
    void equalsReturnsFalseForNull() {
        ProviderFunctionDescriptor desc = new ProviderFunctionDescriptor("func", "1.0.0");

        assertFalse(desc.equals(null));
    }

    @Test
    void equalsReturnsTrueForSameObject() {
        ProviderFunctionDescriptor desc = new ProviderFunctionDescriptor("func", "1.0.0");

        assertTrue(desc.equals(desc));
    }

    @Test
    void canExtendProviderFunctionDescriptor() {
        class CustomDescriptor extends ProviderFunctionDescriptor {
            private String customField;

            CustomDescriptor(String id, String version) {
                super(id, version);
            }

            void setCustomField(String value) {
                customField = value;
            }

            String getCustomField() {
                return customField;
            }
        }

        CustomDescriptor custom = new CustomDescriptor("custom", "1.0");
        custom.setCustomField("custom-value");
        assertEquals("custom-value", custom.getCustomField());
        assertEquals("custom", custom.getId());
    }
}
