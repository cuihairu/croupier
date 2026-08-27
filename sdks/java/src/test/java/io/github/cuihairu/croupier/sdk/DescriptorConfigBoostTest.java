package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.InvokeOptions;
import io.github.cuihairu.croupier.sdk.invoker.JsonSchemaValidator;
import io.github.cuihairu.croupier.sdk.invoker.RetryConfig;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Second coverage boost: descriptor/config construction, defaults,
 * validation helpers and extra schema-validator corners.
 */
@DisplayName("Descriptor, config and validator boost")
class DescriptorConfigBoostTest {

    // -----------------------------------------------------------------------
    // FunctionDescriptor
    // -----------------------------------------------------------------------

    @Test
    @DisplayName("descriptor constructor stores id/version and defaults enabled")
    void descriptorConstructor() {
        FunctionDescriptor descriptor = new FunctionDescriptor("player.ban", "1.2.3");
        assertEquals("player.ban", descriptor.getId());
        assertEquals("1.2.3", descriptor.getVersion());
        assertTrue(descriptor.isEnabled(), "enabled must default to true");
        assertNotNull(descriptor.getTags(), "tags must default to an empty list");
        assertTrue(descriptor.getTags().isEmpty());
    }

    @Test
    @DisplayName("copy constructor deep-copies tags")
    void descriptorCopyConstructor() {
        FunctionDescriptor original = new FunctionDescriptor("fn", "1.0.0");
        original.setTags(List.of("a", "b"));
        original.setResource("player");

        FunctionDescriptor copy = new FunctionDescriptor(original);
        assertEquals(original.getId(), copy.getId());
        assertEquals(List.of("a", "b"), copy.getTags());
        assertNotSame(original.getTags(), copy.getTags(), "tags must be copied");

        copy.getTags().add("c");
        assertEquals(2, original.getTags().size());
    }

    @Test
    @DisplayName("builder fluent chain populates every field")
    void builderFluentChain() {
        FunctionDescriptor descriptor = CroupierSDK.functionDescriptor("mail.send", "2.0.0")
            .resource("mail")
            .operation("send")
            .tags(List.of("gm", "comm"))
            .summary("Send mail")
            .description("Sends in-game mail")
            .operationId("sendMail")
            .build();

        assertEquals("mail.send", descriptor.getId());
        assertEquals("2.0.0", descriptor.getVersion());
        assertEquals("mail", descriptor.getResource());
        assertEquals("send", descriptor.getOperation());
        assertEquals(List.of("gm", "comm"), descriptor.getTags());
        assertEquals("Send mail", descriptor.getSummary());
        assertEquals("Sends in-game mail", descriptor.getDescription());
        assertEquals("sendMail", descriptor.getOperationId());
    }

    // -----------------------------------------------------------------------
    // ClientConfig
    // -----------------------------------------------------------------------

    @Test
    @DisplayName("client config defaults favour local development")
    void clientConfigDefaults() {
        ClientConfig config = new ClientConfig();
        assertEquals("127.0.0.1:19091", config.getAgentAddr());
        assertEquals("development", config.getEnv());
        assertTrue(config.isInsecure());
        assertEquals(30, config.getTimeoutSeconds());
        assertNull(config.getServiceId(), "service id starts unset and must be provided");
    }

    @Test
    @DisplayName("client config constructor args win")
    void clientConfigServiceIdentity() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        assertEquals("game-1", config.getGameId());
        assertEquals("svc-1", config.getServiceId());
    }

    // -----------------------------------------------------------------------
    // InvokeOptions / RetryConfig builders
    // -----------------------------------------------------------------------

    @Test
    @DisplayName("invoke options builder keeps explicit values")
    void invokeOptionsBuilder() {
        InvokeOptions options = InvokeOptions.builder()
            .idempotencyKey("idem-1")
            .timeout(12_000)
            .header("X-Custom", "v")
            .build();

        assertEquals("idem-1", options.getIdempotencyKey());
        assertEquals(12_000, options.getTimeout());
        assertEquals("v", options.getHeaders().get("X-Custom"));
    }

    @Test
    @DisplayName("create() yields empty defaults")
    void invokeOptionsCreate() {
        InvokeOptions options = InvokeOptions.create();
        assertNull(options.getIdempotencyKey());
        assertNull(options.getTimeout());
        assertTrue(options.getHeaders().isEmpty());
    }

    @Test
    @DisplayName("retry config defaults match the other SDKs")
    void retryConfigDefaults() {
        RetryConfig retry = RetryConfig.createDefault();
        assertTrue(retry.isEnabled());
        assertEquals(3, retry.getMaxAttempts());
        assertEquals(100, retry.getInitialDelayMs());
        assertEquals(5000, retry.getMaxDelayMs());
        assertEquals(2.0, retry.getBackoffMultiplier());
        assertEquals(0.1, retry.getJitterFactor());
        assertEquals(List.of(14, 13, 2, 10, 4), retry.getRetryableStatusCodes());
    }

    @Test
    @DisplayName("retry config equals/hashCode/toString round out the value object")
    void retryValueObject() {
        RetryConfig first = RetryConfig.builder().maxAttempts(5).build();
        RetryConfig second = RetryConfig.builder().maxAttempts(5).build();
        assertEquals(first, second);
        assertEquals(first.hashCode(), second.hashCode());
        assertTrue(first.toString().contains("5"));
        assertNotEquals(first, RetryConfig.builder().maxAttempts(6).build());
    }

    // -----------------------------------------------------------------------
    // JsonSchemaValidator corners
    // -----------------------------------------------------------------------

    @Test
    @DisplayName("validator accepts type unions")
    void validatorTypeUnions() {
        Map<String, Object> schema = Map.of("type", List.of("string", "null"));
        assertTrue(JsonSchemaValidator.isValid(schema, "text"));
        assertTrue(JsonSchemaValidator.isValid(schema, null));
        assertFalse(JsonSchemaValidator.isValid(schema, 42L));
    }

    @Test
    @DisplayName("validator accepts boolean-true schemas and ignores unknown types")
    void validatorLenientSchemas() {
        assertTrue(JsonSchemaValidator.isValid(Map.of(), 42L));
        assertTrue(JsonSchemaValidator.isValid(Map.of("type", "weird-type"), 42L));
    }

    @Test
    @DisplayName("validator resolves chained local refs")
    void validatorChainedRefs() {
        Map<String, Object> schema = Map.of(
            "definitions", Map.of(
                "name", Map.of("type", "string", "minLength", 2),
                "person", Map.of("type", "object",
                    "properties", Map.of("name", Map.of("$ref", "#/definitions/name")),
                    "required", List.of("name"))),
            "$ref", "#/definitions/person");

        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("name", "ab")));
        assertFalse(JsonSchemaValidator.isValid(schema, Map.of("name", "a")));
        assertFalse(JsonSchemaValidator.isValid(schema, Map.of()));
    }

    @Test
    @DisplayName("validator reports unresolved refs instead of throwing")
    void validatorUnresolvedRef() {
        Map<String, Object> schema = Map.of("$ref", "#/definitions/missing");
        List<String> errors = JsonSchemaValidator.validate(schema, "anything");
        assertEquals(1, errors.size());
        assertTrue(errors.get(0).contains("unresolved $ref"));
    }

    @Test
    @DisplayName("validator applies array item schemas through uniqueItems")
    void validatorArrayItemsAndUnique() {
        Map<String, Object> schema = Map.of(
            "type", "array",
            "items", Map.of("type", "integer"),
            "uniqueItems", true,
            "minItems", 2);

        assertTrue(JsonSchemaValidator.isValid(schema, List.of(1L, 2L)));
        assertFalse(JsonSchemaValidator.isValid(schema, List.of(1L, 1L)));
        assertFalse(JsonSchemaValidator.isValid(schema, List.of(1L, "x")));
        assertFalse(JsonSchemaValidator.isValid(schema, List.of(1L)));
    }

    @Test
    @DisplayName("validator string constraints combine")
    void validatorStringConstraints() {
        Map<String, Object> schema = Map.of(
            "type", "string", "minLength", 2L, "maxLength", 4L, "pattern", "^[a-z]+$");

        assertTrue(JsonSchemaValidator.isValid(schema, "abc"));
        assertFalse(JsonSchemaValidator.isValid(schema, "a"));
        assertFalse(JsonSchemaValidator.isValid(schema, "abcde"));
        assertFalse(JsonSchemaValidator.isValid(schema, "AB"));
    }

    @Test
    @DisplayName("validator treats integral doubles as integers")
    void validatorIntegralDoubles() {
        Map<String, Object> schema = Map.of("type", "integer");
        assertTrue(JsonSchemaValidator.isValid(schema, 4.0));
        assertFalse(JsonSchemaValidator.isValid(schema, 4.5));
    }

    // -----------------------------------------------------------------------
    // OpenAPI helper semantics
    // -----------------------------------------------------------------------

    @Test
    @DisplayName("openapi helpers handle blank inputs")
    void openapiHelperBlanks() {
        Map<String, Object> empty = Map.of();
        assertEquals("unknown.function", OpenAPIImporter.deriveOperationId(empty, ""));
        assertEquals("Unnamed Function", OpenAPIImporter.deriveSummary(empty, "unknown.function"));
        assertEquals("", OpenAPIImporter.extractExtension(empty, "x-missing"));
        assertEquals("warning", OpenAPIImporter.parseRiskLevel(""));
    }

    @Test
    @DisplayName("openapi title-case handles underscores and mixed case")
    void openapiTitleCase() {
        assertEquals("Player Ban", OpenAPIImporter.toTitleCase("player_ban"));
        assertEquals("A B C", OpenAPIImporter.toTitleCase("a_b_c"));
        assertEquals("Upper", OpenAPIImporter.toTitleCase("UPPER"));
        assertEquals("X", OpenAPIImporter.toTitleCase("x"));
    }
}
