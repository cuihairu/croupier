package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.JsonSchemaValidator;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Third-wave small fillers across pure helpers. */
class PureHelperBranchTest {

    @Test
    void normalizeHintKeyRejectsTwoCharNonPrefixKeys() {
        // "xy": first is x but neither '-' nor '_' follows -> rejected
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(new FunctionDescriptor("f", "1"), "p", "xy", 1));
        // single "a-": first char not x -> rejected
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(new FunctionDescriptor("f", "1"), "p", "a-", 1));
    }

    @Test
    void numericBoundsCoverInstanceofEdges() {
        // exclusive* as non-numbers must be ignored
        Map<String, Object> schema = new HashMap<>();
        schema.put("exclusiveMinimum", "3");
        schema.put("exclusiveMaximum", "9");
        assertTrue(JsonSchemaValidator.isValid(schema, Double.valueOf(5)));

        // minimum as number against a smaller value (already covered) and the
        // exact boundary (equal) hits the `<` false edge
        assertTrue(JsonSchemaValidator.isValid(Map.of("minimum", 5), Double.valueOf(5)));
        assertFalse(JsonSchemaValidator.isValid(Map.of("minimum", 5), Double.valueOf(4.9)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("maximum", 5), Double.valueOf(5)));
        assertFalse(JsonSchemaValidator.isValid(Map.of("maximum", 5), Double.valueOf(5.1)));
    }

    @Test
    void additionalPropertiesWithNonStringDeclaredKeys() {
        // declared set building skips non-string keys (direct Java map only)
        Map<String, Object> schema = new HashMap<>();
        Map<Object, Object> props = new HashMap<>();
        props.put("ok", Map.of("type", "number"));
        props.put(11, Map.of("type", "number"));
        schema.put("properties", props);
        schema.put("additionalProperties", false);
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("ok", 1)));
        assertFalse(JsonSchemaValidator.isValid(schema, Map.of("ok", 1, "extra", 2)));
    }

    @Test
    void enumListIgnoresNonComparableShapes() {
        // enum entry list with map entries compares via map equality
        Map<String, Object> schema = new HashMap<>();
        schema.put("enum", List.of(Map.of("a", 1)));
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("a", 1)));
        assertFalse(JsonSchemaValidator.isValid(schema, "x"));
    }
}
