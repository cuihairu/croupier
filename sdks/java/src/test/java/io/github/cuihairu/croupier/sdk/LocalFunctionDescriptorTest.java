package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class LocalFunctionDescriptorTest {

    @Test
    void defaultConstructorCreatesEmptyDescriptor() {
        LocalFunctionDescriptor desc = new LocalFunctionDescriptor();

        assertNotNull(desc);
    }

    @Test
    void constructorWithIdAndVersion() {
        LocalFunctionDescriptor desc = new LocalFunctionDescriptor("local-func", "1.0.0");

        assertEquals("local-func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
    }

    @Test
    void settersWork() {
        LocalFunctionDescriptor desc = new LocalFunctionDescriptor("f", "1.0");

        desc.setId("updated-f");
        assertEquals("updated-f", desc.getId());

        desc.setVersion("3.0.0");
        assertEquals("3.0.0", desc.getVersion());
    }

    @Test
    void toStringContainsIdAndVersion() {
        LocalFunctionDescriptor desc = new LocalFunctionDescriptor("test-local", "1.0.0");
        String str = desc.toString();

        assertTrue(str.contains("test-local"));
        assertTrue(str.contains("1.0.0"));
    }

    @Test
    void equalsReturnsTrueForSameValues() {
        LocalFunctionDescriptor desc1 = new LocalFunctionDescriptor("func", "1.0.0");
        LocalFunctionDescriptor desc2 = new LocalFunctionDescriptor("func", "1.0.0");

        assertEquals(desc1, desc2);
        assertEquals(desc1.hashCode(), desc2.hashCode());
    }

    @Test
    void equalsReturnsFalseForDifferentValues() {
        LocalFunctionDescriptor desc1 = new LocalFunctionDescriptor("func1", "1.0.0");
        LocalFunctionDescriptor desc2 = new LocalFunctionDescriptor("func2", "1.0.0");

        assertNotEquals(desc1, desc2);
    }

    @Test
    void equalsReturnsFalseForNull() {
        LocalFunctionDescriptor desc = new LocalFunctionDescriptor("func", "1.0.0");

        assertFalse(desc.equals(null));
    }

    @Test
    void equalsReturnsTrueForSameObject() {
        LocalFunctionDescriptor desc = new LocalFunctionDescriptor("func", "1.0.0");

        assertTrue(desc.equals(desc));
    }

    @Test
    void canExtendLocalFunctionDescriptor() {
        class CustomDescriptor extends LocalFunctionDescriptor {
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
