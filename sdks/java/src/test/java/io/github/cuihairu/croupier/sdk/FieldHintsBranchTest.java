package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for FieldHints schema merging. */
class FieldHintsBranchTest {

    private static FunctionDescriptor descriptorWithSchema(String schema) {
        FunctionDescriptor descriptor = new FunctionDescriptor();
        descriptor.setId("fn");
        descriptor.setInputSchema(schema);
        return descriptor;
    }

    @Test
    void rejectsBadArguments() {
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(null, "field", "x-widget", "slider"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), null, "x-widget", "slider"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), "  ", "x-widget", "slider"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), "field", "not-x", "slider"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), "field", null, "slider"));
    }

    @Test
    void normalizesXPrefixVariants() {
        FunctionDescriptor dashed = FieldHints.setFieldHint(
            descriptorWithSchema("{}"), "player", "x-widget", "slider");
        assertTrue(dashed.getInputSchema().contains("x-widget"));

        FunctionDescriptor underscore = FieldHints.setFieldHint(
            descriptorWithSchema("{}"), "player", "x_widget", "slider");
        assertTrue(underscore.getInputSchema().contains("x-widget"));

        // short or wrong-prefix keys are rejected
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), "player", "x", "v"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), "player", "y-widget", "v"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("{}"), "player", "", "v"));
    }

    @Test
    void mergesIntoExistingPropertiesAndFieldNodes() {
        String existing = "{\"type\":\"object\",\"properties\":{\"player\":{\"type\":\"string\",\"x-widget\":\"text\"}}}";
        FunctionDescriptor merged = FieldHints.setFieldHint(
            descriptorWithSchema(existing), "player", "x-widget", "select");
        Map<?, ?> parsed = (Map<?, ?>) io.github.cuihairu.croupier.sdk.invoker.Json.parse(merged.getInputSchema());
        Map<?, ?> properties = (Map<?, ?>) parsed.get("properties");
        Map<?, ?> player = (Map<?, ?>) properties.get("player");
        assertEquals("select", player.get("x-widget"));
        assertEquals("string", player.get("type"));
    }

    @Test
    void replacesNonMapPropertiesAndFieldNodes() {
        // properties present but not a map -> replaced
        FunctionDescriptor replacedProps = FieldHints.setFieldHint(
            descriptorWithSchema("{\"type\":\"object\",\"properties\":\"broken\"}"), "any", "x-widget", 1);
        assertNotNull(replacedProps);

        // field node present but not a map -> replaced
        FunctionDescriptor replacedField = FieldHints.setFieldHint(
            descriptorWithSchema("{\"properties\":{\"any\":\"broken\"}}"), "any", "x-widget", 1);
        Map<?, ?> parsed = (Map<?, ?>) io.github.cuihairu.croupier.sdk.invoker.Json.parse(replacedField.getInputSchema());
        Map<?, ?> player = (Map<?, ?>) ((Map<?, ?>) parsed.get("properties")).get("any");
        assertEquals(Double.valueOf(1.0), player.get("x-widget"));
    }

    @Test
    void rejectsNonObjectAndInvalidSchemas() {
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("[1,2]"), "f", "x-widget", 1));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptorWithSchema("not-json"), "f", "x-widget", 1));
        // null / blank schema falls back to a fresh object schema
        FunctionDescriptor fromNull = FieldHints.setFieldHint(descriptorWithSchema(null), "f", "x-widget", 1);
        assertTrue(fromNull.getInputSchema().contains("x-widget"));
        FunctionDescriptor fromBlank = FieldHints.setFieldHint(descriptorWithSchema("   "), "f", "x-widget", 1);
        assertTrue(fromBlank.getInputSchema().contains("x-widget"));
    }
}
