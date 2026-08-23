package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.JsonSchemaValidator;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for OpenAPI import (Go RegisterFromOpenAPI parity) and the local
 * Draft-07 subset JSON Schema validator.
 */
@DisplayName("OpenAPI import and JSON Schema validation")
class OpenAPIImporterTest {

    /** Records registrations without opening connections. */
    private static final class RecordingClient implements CroupierClient {
        final List<Map.Entry<FunctionDescriptor, FunctionHandler>> registered = new ArrayList<>();
        String failId;

        @Override
        public void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) {
            if (failId != null && failId.equals(descriptor.getId())) {
                throw new IllegalStateException("rejected");
            }
            registered.add(Map.entry(descriptor, handler));
        }

        // Unused client surface ------------------------------------------------
        @Override public java.util.concurrent.CompletableFuture<Void> connect() { return java.util.concurrent.CompletableFuture.completedFuture(null); }
        @Override public void serve() { }
        @Override public java.util.concurrent.CompletableFuture<Void> serveAsync() { return java.util.concurrent.CompletableFuture.completedFuture(null); }
        @Override public void stop() { }
        @Override public void close() { }
        @Override public boolean isConnected() { return false; }
        @Override public String getSessionId() { return ""; }
        @Override public boolean isServing() { return false; }
        @Override public String startTask(String functionId, String payload) { return ""; }
        @Override public String startTask(String functionId, String payload, Map<String, String> metadata) { return ""; }
        @Override public org.reactivestreams.Publisher<io.github.cuihairu.croupier.sdk.invoker.TaskEventInfo> streamTask(String taskId) { return null; }
        @Override public boolean cancelTask(String taskId) { return true; }
    }

    private static FunctionHandler noop() {
        return (context, payload) -> "{}";
    }

    private static String spec() {
        return """
            {
              "openapi": "3.0.3",
              "info": {"title": "GM API", "version": "1.0.0"},
              "paths": {
                "/players/{id}/ban": {
                  "put": {
                    "operationId": "player_ban",
                    "summary": "Ban player",
                    "description": "Bans a player account",
                    "tags": ["gm", "risk"],
                    "x-resource": "player",
                    "x-operation": "ban",
                    "x-permission": "player.ban",
                    "x-risk": "high",
                    "requestBody": {"content": {"application/json": {"schema": {
                      "type": "object",
                      "required": ["playerId", "reason"],
                      "properties": {
                        "playerId": {"type": "string", "description": "Player ID"},
                        "reason": {"type": "string"}
                      }
                    }}}},
                    "responses": {"200": {"content": {"application/json": {"schema": {
                      "type": "object",
                      "properties": {"ok": {"type": "boolean"}}
                    }}}}}
                  }
                },
                "/players/search": {
                  "get": {
                    "tags": ["query"],
                    "responses": {"200": {"content": {"application/json": {"schema": {"type": "array"}}}}}
                  }
                }
              }
            }
            """;
    }

    @Test
    @DisplayName("registers every operation and returns ids")
    void registersAllOperations() throws Exception {
        RecordingClient client = new RecordingClient();
        List<String> registered = OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec(), null, Map.of("player_ban", noop(), "players.search", noop()));
        assertEquals(List.of("player_ban", "players.search"), registered);
        assertEquals(2, client.registered.size());
    }

    @Test
    @DisplayName("maps operation metadata onto descriptors")
    void mapsMetadata() throws Exception {
        RecordingClient client = new RecordingClient();
        OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec(), null, Map.of("player_ban", noop(), "players.search", noop()));

        FunctionDescriptor descriptor = client.registered.get(0).getKey();
        assertEquals("player_ban", descriptor.getId());
        assertEquals("Ban player", descriptor.getSummary());
        assertEquals("Bans a player account", descriptor.getDescription());
        assertEquals(List.of("gm", "risk"), descriptor.getTags());
        assertEquals("player", descriptor.getResource());
        assertEquals("ban", descriptor.getOperation());
        assertEquals("player.ban", descriptor.getPermission());
        assertEquals("high", descriptor.getRisk());
    }

    @Test
    @DisplayName("converts request and response schemas")
    void convertsSchemas() throws Exception {
        RecordingClient client = new RecordingClient();
        OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec(), null, Map.of("player_ban", noop(), "players.search", noop()));

        FunctionDescriptor descriptor = client.registered.get(0).getKey();
        assertTrue(descriptor.getInputSchema().contains("\"required\":[\"playerId\",\"reason\"]"));
        assertTrue(descriptor.getInputSchema().contains("Player ID"));
        assertTrue(descriptor.getOutputSchema().contains("\"type\":\"boolean\""));
    }

    @Test
    @DisplayName("derives ids and defaults when operationId is missing")
    void derivesIds() throws Exception {
        RecordingClient client = new RecordingClient();
        OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec(), null, Map.of("player_ban", noop(), "players.search", noop()));

        FunctionDescriptor descriptor = client.registered.get(1).getKey();
        assertEquals("players.search", descriptor.getId());
        assertEquals("Players.search", descriptor.getSummary());
        assertEquals("medium", descriptor.getRisk());
    }

    @Test
    @DisplayName("applies resource and tag prefixes")
    void appliesPrefixes() throws Exception {
        RecordingClient client = new RecordingClient();
        OpenAPIImporter.ImportOptions options = new OpenAPIImporter.ImportOptions()
            .resourcePrefix("game")
            .tagPrefix("svc-");
        OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec(), options, Map.of("player_ban", noop(), "players.search", noop()));

        FunctionDescriptor descriptor = client.registered.get(0).getKey();
        assertEquals("game.player", descriptor.getResource());
        assertEquals(List.of("svc-gm", "svc-risk"), descriptor.getTags());
    }

    @Test
    @DisplayName("missing handler fails unless continueOnError is set")
    void missingHandler() throws Exception {
        RecordingClient client = new RecordingClient();
        CroupierException missing = assertThrows(CroupierException.class,
            () -> OpenAPIImporter.registerFromOpenAPIWithHandlers(client, spec(), null, Map.of()));
        assertTrue(missing.getMessage().contains("no handler provided for function: player_ban"));

        RecordingClient lenient = new RecordingClient();
        List<String> registered = OpenAPIImporter.registerFromOpenAPIWithHandlers(
            lenient, spec(), new OpenAPIImporter.ImportOptions().continueOnError(true),
            Map.of("players.search", noop()));
        assertEquals(List.of("players.search"), registered);
    }

    @Test
    @DisplayName("registration failures respect continueOnError")
    void registrationFailures() throws Exception {
        RecordingClient client = new RecordingClient();
        client.failId = "player_ban";
        List<String> registered = OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec(), new OpenAPIImporter.ImportOptions().continueOnError(true),
            Map.of("player_ban", noop(), "players.search", noop()));
        assertEquals(List.of("players.search"), registered);

        RecordingClient strict = new RecordingClient();
        strict.failId = "player_ban";
        assertThrows(CroupierException.class,
            () -> OpenAPIImporter.registerFromOpenAPIWithHandlers(
                strict, spec(), null, Map.of("player_ban", noop(), "players.search", noop())));
    }

    @Test
    @DisplayName("rejects malformed specs")
    void rejectsMalformedSpecs() {
        assertThrows(CroupierException.class,
            () -> OpenAPIImporter.registerFromOpenAPI(new RecordingClient(), "{not json", null, id -> noop()));
        assertThrows(CroupierException.class,
            () -> OpenAPIImporter.registerFromOpenAPI(new RecordingClient(), "{\"openapi\":\"3.0.3\"}", null, id -> noop()));
    }

    @Test
    @DisplayName("helper conversions match Go semantics")
    void helperSemantics() {
        Map<String, Object> operation = new LinkedHashMap<>();
        assertEquals("unknown.function", OpenAPIImporter.deriveOperationId(operation, ""));
        assertEquals("api.players.{id}", OpenAPIImporter.deriveOperationId(operation, "/api/players/{id}"));
        assertEquals("Player Ban", OpenAPIImporter.toTitleCase("player_ban"));
        assertEquals("low", OpenAPIImporter.parseRiskLevel("safe"));
        assertEquals("danger", OpenAPIImporter.parseRiskLevel("critical"));
        assertEquals("medium", OpenAPIImporter.parseRiskLevel("bogus"));
        assertEquals("42", OpenAPIImporter.extractExtension(Map.of("x-n", 42L), "x-n"));
        assertEquals("true", OpenAPIImporter.extractExtension(Map.of("x-b", Boolean.TRUE), "x-b"));
    }

    // -----------------------------------------------------------------------
    // JsonSchemaValidator
    // -----------------------------------------------------------------------

    @Test
    @DisplayName("validator accepts valid payloads")
    void validatorAccepts() {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "required", List.of("a"),
            "properties", Map.of("a", Map.of("type", "integer")));
        assertTrue(JsonSchemaValidator.isValid(
            schema, Map.of("a", Long.valueOf(1))));
    }

    @Test
    @DisplayName("validator reports type, required and nested errors")
    void validatorReports() {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "required", List.of("a"),
            "properties", Map.of("a", Map.of("type", "integer")));
        List<String> errors = JsonSchemaValidator.validate(schema, Map.of("a", "str"));
        assertEquals(1, errors.size());
        assertTrue(errors.get(0).contains("/a"));

        errors = JsonSchemaValidator.validate(schema, Map.of());
        assertFalse(errors.isEmpty());
    }

    @Test
    @DisplayName("validator enforces numeric and string constraints")
    void validatorConstraints() {
        List<String> errors = JsonSchemaValidator.validate(
            Map.of("type", "number", "minimum", 1L, "maximum", 10L),
            Double.valueOf(0.5));
        assertEquals(1, errors.size());

        errors = JsonSchemaValidator.validate(
            Map.of("type", "string", "minLength", 3L, "pattern", "^[a-z]+$"),
            "AB");
        assertEquals(2, errors.size());

        assertTrue(JsonSchemaValidator.isValid(
            Map.of("type", "string", "pattern", "^[a-z]+$"), "abc"));
    }

    @Test
    @DisplayName("validator enforces enum, const and array constraints")
    void validatorCollections() {
        assertFalse(JsonSchemaValidator.isValid(Map.of("enum", List.of("a", "b")), "c"));
        assertTrue(JsonSchemaValidator.isValid(Map.of("const", 7L), 7L));

        List<String> errors = JsonSchemaValidator.validate(
            Map.of("type", "array", "minItems", 2L, "uniqueItems", true, "items", Map.of("type", "integer")),
            List.of(1L));
        assertFalse(errors.isEmpty());

        errors = JsonSchemaValidator.validate(
            Map.of("type", "array", "items", Map.of("type", "integer")),
            List.of(1L, "x"));
        assertTrue(errors.stream().anyMatch(message -> message.contains("/1")));
    }

    @Test
    @DisplayName("validator enforces additionalProperties")
    void validatorAdditionalProperties() {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "properties", Map.of("a", Map.of("type", "string")),
            "additionalProperties", Boolean.FALSE);
        assertTrue(JsonSchemaValidator.isValid(schema, Map.of("a", "x")));
        assertFalse(JsonSchemaValidator.isValid(schema, Map.of("a", "x", "b", "y")));
    }

    @Test
    @DisplayName("validator resolves local $ref pointers")
    void validatorRefs() {
        Map<String, Object> item = Map.of("type", "integer");
        Map<String, Object> schema = Map.of(
            "definitions", Map.of("item", item),
            "type", "array",
            "items", Map.of("$ref", "#/definitions/item"));
        assertTrue(JsonSchemaValidator.isValid(schema, List.of(1L, 2L)));
        assertFalse(JsonSchemaValidator.isValid(schema, List.of(1L, "two")));
    }

    @Test
    @DisplayName("validator accepts integers written as integral doubles")
    void validatorIntegerDoubles() {
        assertTrue(JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.valueOf(3.0)));
        assertFalse(JsonSchemaValidator.isValid(Map.of("type", "integer"), Double.valueOf(3.5)));
    }

    @Test
    @DisplayName("validator validates JSON strings directly")
    void validatorStrings() {
        List<String> errors = JsonSchemaValidator.validate("{\"a\":1}", "{\"type\":\"object\",\"required\":[\"a\"]}");
        assertTrue(errors.isEmpty());
        errors = JsonSchemaValidator.validate("{}", "{\"type\":\"object\",\"required\":[\"a\"]}");
        assertFalse(errors.isEmpty());
    }
}
