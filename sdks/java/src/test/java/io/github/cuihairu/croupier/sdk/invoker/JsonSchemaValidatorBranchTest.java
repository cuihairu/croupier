package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for JsonSchemaValidator edge branches. */
class JsonSchemaValidatorBranchTest {

    private static List<String> errors(String payload, String schema) {
        return JsonSchemaValidator.validate(payload, schema);
    }

    @Test
    void integerTypeAcceptsWholeDoublesAndRejectsFractions() {
        assertTrue(JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.valueOf(3.0)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.valueOf(-8.0)));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.valueOf(3.5)));
        // degenerate doubles short-circuit the NaN / infinite branches
        assertTrue(!JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.NaN));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.POSITIVE_INFINITY));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("type", "integer"), "3"));
    }

    @Test
    void enumComparesNumbersAndMixedShapes() {
        assertTrue(JsonSchemaValidator.isValid(Map.of("enum", List.of(1, 2)), Long.valueOf(2)));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("enum", List.of(1, 2)), Long.valueOf(3)));
        // string vs number: jsonEquals falls back to Objects.equals
        assertTrue(!JsonSchemaValidator.isValid(Map.of("enum", List.of(1, 2)), "2"));
        // mixed-type enum with a string value
        assertTrue(JsonSchemaValidator.isValid(Map.of("enum", List.of("a", 1)), "a"));
        // number value against string-only enum
        assertTrue(!JsonSchemaValidator.isValid(Map.of("enum", List.of("a", "b")), Long.valueOf(1)));
        // const path with equal value
        assertTrue(JsonSchemaValidator.isValid(Map.of("const", 7), Long.valueOf(7)));
    }

    @Test
    void numericChecksIgnoreNonNumberBoundsAndZeroDivisor() {
        Map<String, Object> schema = new HashMap<>();
        schema.put("minimum", "5");
        schema.put("maximum", "10");
        schema.put("exclusiveMinimum", "1");
        schema.put("exclusiveMaximum", "20");
        schema.put("multipleOf", 0);
        assertTrue(JsonSchemaValidator.isValid(schema, Long.valueOf(3)));

        assertTrue(!JsonSchemaValidator.isValid(Map.of("exclusiveMinimum", 5), Long.valueOf(5)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("exclusiveMinimum", 5), Long.valueOf(6)));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("exclusiveMaximum", 5), Long.valueOf(5)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("exclusiveMaximum", 5), Long.valueOf(4)));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("multipleOf", 3), Long.valueOf(4)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("multipleOf", 3), Long.valueOf(9)));
    }

    @Test
    void arrayBoundChecksCoverBothSides() {
        assertTrue(!JsonSchemaValidator.isValid(Map.of("minItems", 2), List.of(1)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("minItems", 1), List.of(1)));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("maxItems", 1), List.of(1, 2)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("maxItems", 2), List.of(1, 2)));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("uniqueItems", true), List.of(1, 1)));
        assertTrue(JsonSchemaValidator.isValid(Map.of("uniqueItems", false), List.of(1, 1)));
    }

    @Test
    void objectChecksCoverMissingPropertiesAndAdditionalVariants() {
        // declared property missing from value: containsKey false branch
        assertTrue(errors("{}", "{\"properties\": {\"a\": {\"type\": \"string\"}}}").isEmpty());

        // non-string property name in schema map (only reachable via direct Java map)
        Map<String, Object> inner = new HashMap<>();
        Map<Object, Object> badProps = new HashMap<>();
        badProps.put(7, Map.of("type", "string"));
        inner.put("properties", badProps);
        assertTrue(JsonSchemaValidator.isValid(inner, Map.of("a", "x")));

        // additionalProperties without properties map
        assertTrue(errors("{\"x\": 1}", "{\"additionalProperties\": false}").isEmpty());

        // additionalProperties as schema applying to undeclared fields
        assertTrue(errors("{\"x\": 1}", "{\"properties\": {}, \"additionalProperties\": {\"type\": \"number\"}}").isEmpty());
        assertTrue(!errors("{\"x\": \"no\"}", "{\"properties\": {}, \"additionalProperties\": {\"type\": \"number\"}}").isEmpty());

        // additionalProperties with a non-boolean, non-map value is ignored
        assertTrue(errors("{\"x\": 1}", "{\"properties\": {}, \"additionalProperties\": \"whatever\"}").isEmpty());
    }

    @Test
    void requiredListIgnoresNonStringNames() {
        Map<String, Object> schema = new HashMap<>();
        schema.put("required", List.of(5, "real"));
        assertTrue(!JsonSchemaValidator.isValid(schema, Map.of()));
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("real", 1)));
    }

    @Test
    void typeListSpecsCoverStringAndNonStringItems() {
        assertTrue(JsonSchemaValidator.isValid(
            Map.of("type", List.of("string", "number")), Long.valueOf(3)));
        assertTrue(JsonSchemaValidator.isValid(
            Map.of("type", List.of("string", "number")), "three"));
        assertTrue(!JsonSchemaValidator.isValid(
            Map.of("type", List.of("string", "number")), List.of()));
        // non-string items in the type list are skipped
        Map<String, Object> mixedTypes = new HashMap<>();
        mixedTypes.put("type", List.of(3, "string"));
        assertTrue(JsonSchemaValidator.isValid(mixedTypes, "ok"));
        assertTrue(!JsonSchemaValidator.isValid(mixedTypes, 3));
        // unknown type name always matches
        assertTrue(JsonSchemaValidator.isValid(Map.of("type", "mystery"), Map.of()));
    }

    @Test
    void patternErrorsAreIgnoredWhenRegexIsInvalid() {
        assertTrue(JsonSchemaValidator.isValid(Map.of("pattern", "([unclosed"), "any"));
        assertTrue(!JsonSchemaValidator.isValid(Map.of("pattern", "^ab"), "xyz"));
        assertTrue(JsonSchemaValidator.isValid(Map.of("pattern", "^ab"), "abc"));
        assertTrue(JsonSchemaValidator.isValid(Map.of("pattern", 7), "abc"));
    }

    @Test
    void refResolutionCoversUnresolvableAndNestedPointers() {
        assertTrue(!errors("{\"a\": 1}", "{\"$ref\": \"#/definitions/missing\"}").isEmpty());
        assertTrue(errors("1", "{\"$ref\": \"#/defs/num\", \"defs\": {\"num\": {\"type\": \"number\"}}}").isEmpty());
        assertTrue(!errors("\"x\"", "{\"$ref\": \"#/defs/num\", \"defs\": {\"num\": {\"type\": \"number\"}}}").isEmpty());
        // non-string / non-pointer refs fall through to normal validation
        assertTrue(JsonSchemaValidator.isValid(
            Map.of("$ref", 3, "type", "object"),
            Map.of("a", 1)));
        assertTrue(JsonSchemaValidator.isValid(
            Map.of("$ref", "http://remote"),
            Map.of("a", 1)));
        // boolean schema is treated as pass
        assertTrue(JsonSchemaValidator.isValid(Boolean.TRUE, Map.of("a", 1)));
    }
}
