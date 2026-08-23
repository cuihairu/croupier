package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Third coverage boost: JsonSchemaValidator branch corners (non-map schemas,
 * number/boolean/null type names, malformed keyword values, empty pointers)
 * and wire codec edge behaviour.
 */
@DisplayName("JsonSchemaValidator branch corners")
class JsonSchemaValidatorBoostTest {

    private static Object parse(String json) {
        return Json.parse(json);
    }

    private static List<String> validate(String schemaJson, String payloadJson) {
        return JsonSchemaValidator.validate(parse(schemaJson), parse(payloadJson));
    }

    @Test
    @DisplayName("non-map schemas are treated as lenient")
    void nonMapSchemasPass() {
        assertTrue(JsonSchemaValidator.isValid(Boolean.TRUE, 42L));
        assertTrue(JsonSchemaValidator.isValid("not-a-schema", 42L));
        assertTrue(JsonSchemaValidator.isValid(null, 42L));
    }

    @Test
    @DisplayName("type list entries that are not strings are ignored")
    void typeListFiltersNonStrings() {
        Object schema = Map.of("type", List.of(42L, true, "null"));
        // The only valid candidate is "null", so 42 must fail and null must pass.
        assertFalse(JsonSchemaValidator.isValid(schema, 42L));
        assertTrue(JsonSchemaValidator.isValid(schema, null));
    }

    @Test
    @DisplayName("empty type list is ignored entirely")
    void emptyTypeListIgnored() {
        Object schema = Map.of("type", List.of());
        assertTrue(JsonSchemaValidator.isValid(schema, "anything"));
    }

    @Test
    @DisplayName("type given as a number is ignored")
    void nonStringTypeIgnored() {
        Object schema = Map.of("type", 7L);
        assertTrue(JsonSchemaValidator.isValid(schema, "anything"));
    }

    @Test
    @DisplayName("unknown json type names report the fallback branch")
    void jsonTypeNameFallback() {
        List<String> errors = JsonSchemaValidator.validate(
            Map.of("type", "object"), "just-a-string-at-root");
        // "string" maps fine; ensure list of unknown-type messages carries type info
        assertFalse(errors.isEmpty());
        assertTrue(errors.get(0).contains("expected type"));
    }

    @Test
    @DisplayName("malformed numeric keywords are ignored instead of throwing")
    void malformedNumericKeywordsIgnored() {
        Map<String, Object> schema = new HashMap<>();
        schema.put("type", "number");
        schema.put("minimum", "not-a-number");
        schema.put("maximum", List.of());
        schema.put("multipleOf", 0L);
        assertTrue(JsonSchemaValidator.isValid(schema, 123.45));
    }

    @Test
    @DisplayName("exclusive bounds and multipleOf report precise messages")
    void exclusiveBoundsAndMultipleOf() {
        List<String> errors = validate(
            "{\"type\":\"number\",\"exclusiveMinimum\":5,\"exclusiveMaximum\":10,\"multipleOf\":3}",
            "5");
        // 5 violates exclusiveMinimum; also not a multiple of 3.
        assertEquals(2, errors.size());
        assertTrue(errors.get(0).contains("must be greater than 5"));

        errors = validate(
            "{\"type\":\"number\",\"exclusiveMaximum\":10}", "10");
        assertEquals(1, errors.size());
        assertTrue(errors.get(0).contains("must be less than 10"));
    }

    @Test
    @DisplayName("malformed string keywords are ignored")
    void malformedStringKeywordsIgnored() {
        Map<String, Object> schema = new HashMap<>();
        schema.put("type", "string");
        schema.put("minLength", "nope");
        schema.put("maxLength", Map.of());
        schema.put("pattern", 42L);
        assertTrue(JsonSchemaValidator.isValid(schema, "any text"));
    }

    @Test
    @DisplayName("invalid regex patterns are ignored")
    void invalidPatternIgnored() {
        Map<String, Object> schema = Map.of("type", "string", "pattern", "([unclosed");
        assertTrue(JsonSchemaValidator.isValid(schema, "anything"));
    }

    @Test
    @DisplayName("malformed array keywords are ignored")
    void malformedArrayKeywordsIgnored() {
        Map<String, Object> schema = new HashMap<>();
        schema.put("type", "array");
        schema.put("minItems", "many");
        schema.put("maxItems", "few");
        schema.put("uniqueItems", "yes");
        assertTrue(JsonSchemaValidator.isValid(schema, List.of(1L, 1L, 1L, 1L)));
    }

    @Test
    @DisplayName("non-map items schemas are ignored")
    void nonMapItemsIgnored() {
        Map<String, Object> schema = Map.of("type", "array", "items", List.of());
        assertTrue(JsonSchemaValidator.isValid(schema, List.of(1L, "mixed")));
        assertTrue(JsonSchemaValidator.isValid(schema, List.of()));
    }

    @Test
    @DisplayName("non-list required entries are ignored")
    void nonStringRequiredEntriesIgnored() {
        Map<String, Object> schema = Map.of("type", "object", "required", List.of(42L, "name"));
        List<String> errors = JsonSchemaValidator.validate(schema, Map.of());
        assertEquals(1, errors.size());
        assertTrue(errors.get(0).contains("'name'"));
    }

    @Test
    @DisplayName("non-map properties entries are skipped")
    void nonMapPropertySchemasSkipped() {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "properties", Map.of("good", Map.of("type", "integer"), "bad", "scalar"));
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("good", 1L, "bad", "x")));
        assertFalse(JsonSchemaValidator.isValid(schema, Map.of("good", "x")));
    }

    @Test
    @DisplayName("additionalProperties as a schema validates unknown fields")
    void additionalPropertiesSchemaForm() {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "properties", Map.of(),
            "additionalProperties", Map.of("type", "integer"));
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("any", 5L)));
        List<String> errors = JsonSchemaValidator.validate(schema, Map.of("any", "str"));
        assertEquals(1, errors.size());
        assertTrue(errors.get(0).contains("/any"));
    }

    @Test
    @DisplayName("malformed additionalProperties values are ignored")
    void malformedAdditionalPropertiesIgnored() {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "properties", Map.of("a", Map.of()),
            "additionalProperties", "yes");
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("a", 1L, "extra", "x")));
    }

    @Test
    @DisplayName("refs pointing into non-map nodes and empty segments resolve safely")
    void refPointerEdgeCases() {
        // Pointer into a scalar resolves to a non-map schema, which is lenient.
        Map<String, Object> scalar = new HashMap<>();
        scalar.put("definitions", Map.of("x", 5L));
        scalar.put("$ref", "#/definitions/x");
        assertTrue(JsonSchemaValidator.isValid(scalar, 1L));

        // Pointer that leaves the object tree (missing key) is unresolved.
        Map<String, Object> missing = new HashMap<>();
        missing.put("definitions", Map.of());
        missing.put("$ref", "#/definitions/nope");
        List<String> errors = JsonSchemaValidator.validate(missing, 1L);
        assertEquals(1, errors.size());
        assertTrue(errors.get(0).contains("unresolved $ref"));

        // Non-local refs fall through to keyword validation.
        Map<String, Object> remote = Map.of("$ref", "https://example.com/schema.json", "type", "integer");
        assertTrue(JsonSchemaValidator.isValid(remote, 1L));
        assertFalse(JsonSchemaValidator.isValid(remote, "x"));
    }

    @Test
    @DisplayName("enum compares numbers numerically and other types structurally")
    void enumComparisonSemantics() {
        Map<String, Object> intEnum = Map.of("enum", List.of(1L, 2L));
        assertTrue(JsonSchemaValidator.isValid(intEnum, 2.0)); // 2.0 == 2 numerically
        assertFalse(JsonSchemaValidator.isValid(intEnum, 3L));

        Map<String, Object> stringEnum = Map.of("enum", List.of("a", "b"));
        assertFalse(JsonSchemaValidator.isValid(stringEnum, "A")); // case-sensitive
    }

    @Test
    @DisplayName("wire codec: ProviderConnectRequest with all scalar fields round-trips")
    void wireProviderConnectScalarFields() {
        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.2.3", "rpc-addr", List.of(),
            "java", "9.9.9", "croupier", "2.0");
        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(request);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        // rpcAddr is not part of the wire format; decode always yields "".
        assertEquals("", decoded.rpcAddr);
        assertEquals("svc", decoded.serviceId);
        assertEquals("1.2.3", decoded.version);
        assertEquals("java", decoded.sdkLanguage);
        assertEquals("9.9.9", decoded.sdkVersion);
        assertEquals("croupier", decoded.sdkName);
        assertEquals("2.0", decoded.protocolVersion);
        assertTrue(decoded.functions.isEmpty());
    }

    @Test
    @DisplayName("wire codec: descriptors with only tags and schemas round-trip")
    void wireDescriptorTagsAndSchemasOnly() {
        SdkWireMessages.ProviderFunctionDescriptor descriptor =
            new SdkWireMessages.ProviderFunctionDescriptor(
                "fn.tags", "1.0.0", List.of("one", "two"), null, null, null, false,
                "{}", "[]", null, null, null, null, false, null, null, true, null);
        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "", List.of(descriptor));
        SdkWireMessages.ProviderConnectRequest decoded =
            SdkWireMessages.decodeProviderConnectRequest(SdkWireMessages.encodeProviderConnectRequest(request));

        assertEquals(1, decoded.functions.size());
        SdkWireMessages.ProviderFunctionDescriptor fn = decoded.functions.get(0);
        assertEquals(List.of("one", "two"), fn.tags);
        assertEquals("{}", fn.inputSchema);
        assertEquals("[]", fn.outputSchema);
        assertTrue(fn.enabled);
    }

    @Test
    @DisplayName("wire codec: hand-built task event frames decode field by field")
    void wireTaskEventIndividualFields() throws Exception {
        // Hand-encode type (field 1), message (field 2), progress (field 3 varint), payload (field 4 bytes).
        java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream();
        com.google.protobuf.CodedOutputStream coded = com.google.protobuf.CodedOutputStream.newInstance(out);
        coded.writeString(1, "progress");
        coded.writeString(2, "half");
        coded.writeInt32(3, 50);
        coded.writeByteArray(4, new byte[]{9, 9});
        coded.flush();
        SdkWireMessages.TaskEvent event = SdkWireMessages.decodeTaskEvent(out.toByteArray());

        assertEquals("progress", event.type);
        assertEquals("half", event.message);
        assertEquals(50, event.progress);
        assertArrayEquals(new byte[]{9, 9}, event.payload);
    }
}
