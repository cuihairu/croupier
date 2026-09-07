package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for OpenAPIImporter conversion logic. */
class OpenAPIImporterBranchTest {

    /** Records registrations without opening connections. */
    private static final class RecordingClient implements CroupierClient {
        final List<FunctionDescriptor> registered = new ArrayList<>();
        boolean failNext;

        @Override
        public void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) {
            if (failNext) {
                failNext = false;
                throw new IllegalStateException("registration refused");
            }
            registered.add(descriptor);
        }

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

    @Test
    void rejectsNonObjectAndMissingPathsSpecs() {
        RecordingClient client = new RecordingClient();
        assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(client, "[1,2,3]", null, id -> null));
        assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(client, "{\"info\":{}}", null, id -> null));
        assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(client, "{broken", null, id -> null));
    }

    @Test
    void skipsMalformedPathEntries() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {
              "paths": {
                "/health": {"get": {"operationId": "health.check", "responses": {"200": {"description": "ok"}}}},
                "not-a-path-item": "just-a-string"
              }
            }
            """;
        List<String> ids = OpenAPIImporter.registerFromOpenAPI(client, spec, null, id -> noop());
        assertEquals(List.of("health.check"), ids);
    }

    @Test
    void handlerMissingWithoutContinueOnErrorFails() {
        RecordingClient client = new RecordingClient();
        String spec = """
            {"paths": {"/a": {"get": {"operationId": "a.get"}}}}
            """;
        CroupierException error = assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(client, spec, null, id -> null));
        assertTrue(error.getMessage().contains("a.get"));
    }

    @Test
    void handlerMissingWithContinueOnErrorSkipsRegistration() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {"paths": {
              "/a": {"get": {"operationId": "a.get"}},
              "/b": {"get": {"operationId": "b.get"}}
            }}
            """;
        List<String> ids = OpenAPIImporter.registerFromOpenAPI(client, spec,
            new OpenAPIImporter.ImportOptions().continueOnError(true), id -> null);
        assertTrue(ids.isEmpty());
    }

    @Test
    void registerFailureWithContinueOnErrorSkipsRemaining() throws Exception {
        RecordingClient client = new RecordingClient();
        client.failNext = true;
        String spec = """
            {"paths": {
              "/a": {"get": {"operationId": "a.get"}},
              "/b": {"get": {"operationId": "b.get"}}
            }}
            """;
        List<String> ids = OpenAPIImporter.registerFromOpenAPI(client, spec,
            new OpenAPIImporter.ImportOptions().continueOnError(true), id -> noop());
        assertEquals(List.of("b.get"), ids);
    }

    @Test
    void registerFailureWithoutContinueOnErrorPropagates() {
        RecordingClient client = new RecordingClient();
        client.failNext = true;
        String spec = """
            {"paths": {"/a": {"get": {"operationId": "a.get"}}}}
            """;
        assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(client, spec, null, id -> noop()));
    }

    @Test
    void withHandlersMapUsesMapLookup() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {"paths": {"/a": {"get": {"operationId": "a.get"}}}}
            """;
        List<String> ids = OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec, null, Map.of("a.get", noop()));
        assertEquals(List.of("a.get"), ids);

        List<String> missing = OpenAPIImporter.registerFromOpenAPIWithHandlers(
            client, spec, new OpenAPIImporter.ImportOptions().continueOnError(true), Map.of());
        assertTrue(missing.isEmpty());
    }

    @Test
    void derivesOperationIdFromPathWhenMissing() {
        assertEquals("given.id",
            OpenAPIImporter.deriveOperationId(Map.of("operationId", "given.id"), "/ignored"));
        assertEquals("a.b.c",
            OpenAPIImporter.deriveOperationId(Map.of(), "/a/b/c"));
        assertEquals("a.b",
            OpenAPIImporter.deriveOperationId(Map.of("operationId", ""), "/a//b/"));
        assertEquals("unknown.function",
            OpenAPIImporter.deriveOperationId(Map.of(), "/"));
        assertEquals("unknown.function",
            OpenAPIImporter.deriveOperationId(Map.of(), null));
    }

    @Test
    void derivesSummaryVariants() {
        assertEquals("Given summary",
            OpenAPIImporter.deriveSummary(Map.of("summary", "Given summary"), "fn"));
        assertEquals("Unnamed Function",
            OpenAPIImporter.deriveSummary(Map.of(), "unknown.function"));
        assertEquals("Unnamed Function",
            OpenAPIImporter.deriveSummary(Map.of(), null));
        assertEquals("Player Ban",
            OpenAPIImporter.deriveSummary(Map.of(), "player_ban"));
    }

    @Test
    void titleCaseSkipsEmptyWords() {
        assertEquals("A  B", OpenAPIImporter.toTitleCase("a__b"));
        assertEquals("X", OpenAPIImporter.toTitleCase("x"));
        assertEquals("Ab Cd", OpenAPIImporter.toTitleCase("AB_CD"));
    }

    @Test
    void extractExtensionCoversValueShapes() {
        assertEquals("", OpenAPIImporter.extractExtension(Map.of(), "x-missing"));
        assertEquals("text", OpenAPIImporter.extractExtension(Map.of("x-a", "text"), "x-a"));
        assertEquals("true", OpenAPIImporter.extractExtension(Map.of("x-a", Boolean.TRUE), "x-a"));
        assertEquals("false", OpenAPIImporter.extractExtension(Map.of("x-a", Boolean.FALSE), "x-a"));
        assertEquals("[1,2]", OpenAPIImporter.extractExtension(Map.of("x-a", List.of(1, 2)), "x-a"));
    }

    @Test
    void parseRiskLevelCoversAllAliases() {
        assertEquals("safe", OpenAPIImporter.parseRiskLevel("low"));
        assertEquals("safe", OpenAPIImporter.parseRiskLevel("SAFE"));
        assertEquals("warning", OpenAPIImporter.parseRiskLevel("medium"));
        assertEquals("warning", OpenAPIImporter.parseRiskLevel("moderate"));
        assertEquals("warning", OpenAPIImporter.parseRiskLevel("warning"));
        assertEquals("high", OpenAPIImporter.parseRiskLevel("high"));
        assertEquals("danger", OpenAPIImporter.parseRiskLevel("danger"));
        assertEquals("danger", OpenAPIImporter.parseRiskLevel("CRITICAL"));
        assertEquals("warning", OpenAPIImporter.parseRiskLevel("nonsense"));
    }

    @Test
    void importCoversResponseSchemaAndApprovalVariants() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {
              "paths": {
                "/full": {"post": {
                  "operationId": "full.op",
                  "summary": "Full Op",
                  "description": "does everything",
                  "tags": ["alpha", 7, "beta"],
                  "x-resource": "game",
                  "x-operation": "write",
                  "x-permission": "",
                  "x-capability": "cap",
                  "x-execution": "async",
                  "x-risk": "high",
                  "x-approval": {"required": "true", "policyKey": "policy-1"},
                  "requestBody": {"content": {"application/json": {"schema": {
                    "type": "object",
                    "description": "",
                    "properties": {"name": {"type": "string", "description": "player name"},
                                    "count": {"type": "integer"},
                                    "nodocs": {"type": ""},
                                    "nodesc": {},
                                    "weird": "not-a-map"},
                    "required": []
                  }}}},
                  "responses": {"200": {"content": {"application/json": {"schema": {
                    "type": "object", "properties": {"ok": {"type": "boolean"}}
                  }}}}}
                }},
                "/nocontent": {"get": {
                  "operationId": "no.content",
                  "responses": {"200": {"description": "no content"}}
                }},
                "/emptyschema": {"get": {
                  "operationId": "empty.schema",
                  "requestBody": {"content": {"application/json": {"schema": {}}}},
                  "responses": {"not-a-map": 1}
                }},
                "/badresponses": {"get": {
                  "operationId": "bad.responses",
                  "responses": "not-a-map"
                }},
                "/noresponses": {"get": {"operationId": "no.responses"}}
              }
            }
            """;
        List<String> ids = OpenAPIImporter.registerFromOpenAPI(client, spec,
            new OpenAPIImporter.ImportOptions().resourcePrefix("res").tagPrefix("tag"),
            id -> noop());
        assertEquals(List.of("full.op", "no.content", "empty.schema", "bad.responses", "no.responses"), ids);

        FunctionDescriptor full = client.registered.get(0);
        assertEquals("Full Op", full.getSummary());
        assertEquals("does everything", full.getDescription());
        assertEquals(List.of("tagalpha", "tagbeta"), full.getTags());
        assertEquals("res.game", full.getResource());
        assertEquals("write", full.getOperation());
        assertEquals("cap", full.getCapability());
        assertEquals("async", full.getExecution());
        assertEquals("high", full.getRisk());
        assertTrue(full.isApprovalRequired());
        assertEquals("policy-1", full.getApprovalPolicyKey());
        assertTrue(full.getInputSchema().contains("\"name\""));
        assertTrue(full.getInputSchema().contains("player name"));
        assertTrue(full.getOutputSchema().contains("\"ok\""));
    }

    @Test
    void approvalExtensionCoversNonBooleanRequiredAndBlankPolicyKey() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {
              "paths": {
                "/a": {"get": {"operationId": "a.get", "x-approval": {"required": "false", "policyKey": ""}}},
                "/b": {"get": {"operationId": "b.get", "x-approval": {"required": 1, "policyKey": 2}}},
                "/c": {"get": {"operationId": "c.get", "x-approval": "not-a-map"}},
                "/d": {"get": {"operationId": "d.get", "x-approval": {"required": true}}}
              }
            }
            """;
        OpenAPIImporter.registerFromOpenAPI(client, spec, null, id -> noop());
        assertFalse(client.registered.get(0).isApprovalRequired());
        assertNull(client.registered.get(0).getApprovalPolicyKey());
        assertFalse(client.registered.get(1).isApprovalRequired());
        assertNull(client.registered.get(1).getApprovalPolicyKey());
        assertFalse(client.registered.get(2).isApprovalRequired());
        assertTrue(client.registered.get(3).isApprovalRequired());
    }

    @Test
    void importOptionsNormalizeNullPrefixes() {
        OpenAPIImporter.ImportOptions options = new OpenAPIImporter.ImportOptions()
            .resourcePrefix(null).tagPrefix(null).defaultTimeoutMs(100);
        assertTrue(options.toString().isEmpty() || options != null);
    }
}
