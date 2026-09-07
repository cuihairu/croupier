package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Second-wave branch fillers for OpenAPIImporter edge conditions. */
class OpenAPIImporterEmptyValueBranchTest {

    private static class RecordingClient implements CroupierClient {
        final List<FunctionDescriptor> registered = new ArrayList<>();

        @Override
        public void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) {
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
    void emptySummaryFallsBackToFunctionIdTitle() {
        assertEquals("Fn", OpenAPIImporter.deriveSummary(Map.of("summary", ""), "fn"));
        assertEquals("Fn X", OpenAPIImporter.deriveSummary(Map.of("summary", ""), "fn_x"));
    }

    @Test
    void optionsWithoutContinueOnErrorPropagateHandlerAndRegistrationFailures() {
        RecordingClient client = new RecordingClient();
        String spec = """
            {"paths": {"/a": {"get": {"operationId": "a.get"}}}}
            """;
        // options present but continueOnError=false (the default)
        OpenAPIImporter.ImportOptions options = new OpenAPIImporter.ImportOptions();
        assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(client, spec, options, id -> null));

        RecordingClient failing = new RecordingClient() {
            @Override
            public void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) {
                throw new IllegalStateException("nope");
            }
        };
        assertThrows(CroupierException.class, () ->
            OpenAPIImporter.registerFromOpenAPI(failing, spec, options, id -> noop()));
    }

    @Test
    void schemaConversionCoversEmptyTypeAndDescriptionVariants() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {
              "paths": {
                "/edge": {"post": {
                  "operationId": "edge.op",
                  "requestBody": {"content": {"application/json": {"schema": {
                    "type": "",
                    "description": "",
                    "properties": {
                      "plain": {"type": "", "description": ""},
                      "filled": {"type": "string", "description": "docs"}
                    },
                    "required": ["plain"]
                  }}}},
                  "responses": {"200": {"content": {"application/json": {"schema": {
                    "type": "object"
                  }}}}}
                }}
              }
            }
            """;
        List<String> ids = OpenAPIImporter.registerFromOpenAPI(client, spec, null, id -> noop());
        assertEquals(List.of("edge.op"), ids);
        FunctionDescriptor descriptor = client.registered.get(0);
        assertTrue(descriptor.getInputSchema().contains("plain"));
        assertTrue(descriptor.getInputSchema().contains("docs"));
        assertTrue(descriptor.getInputSchema().contains("required"));
    }

    @Test
    void approvalRequiredEmptyStringIsIgnored() throws Exception {
        RecordingClient client = new RecordingClient();
        String spec = """
            {"paths": {"/a": {"get": {
              "operationId": "a.get",
              "x-approval": {"required": ""}
            }}}}
            """;
        OpenAPIImporter.registerFromOpenAPI(client, spec, null, id -> noop());
        assertEquals(false, client.registered.get(0).isApprovalRequired());
        assertEquals(null, client.registered.get(0).getApprovalPolicyKey());
    }
}
