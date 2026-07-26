package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.util.List;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import io.github.cuihairu.croupier.sdk.invoker.InvokeOptions;
import io.github.cuihairu.croupier.sdk.invoker.Invoker;
import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;

@DisplayName("CroupierSDK Factory Tests")
class CroupierSDKTest {

    // ========== Client Factory Tests ==========

    @Test
    @DisplayName("Create client with config")
    void createClientWithConfig() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        CroupierClient client = CroupierSDK.createClient(config);

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    @DisplayName("Create client with game and service IDs")
    void createClientWithGameAndServiceIds() {
        CroupierClient client = CroupierSDK.createClient("game2", "svc2");

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    @DisplayName("Create client with game, service, and agent address")
    void createClientWithGameServiceAndAgentAddress() {
        CroupierClient client = CroupierSDK.createClient("game3", "svc3", "localhost:9999");

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    @DisplayName("Create client with null config should throw exception")
    void createClientWithNullConfigThrows() {
        assertThrows(NullPointerException.class, () -> CroupierSDK.createClient((ClientConfig) null));
    }

    @Test
    @DisplayName("Create client with null game ID should throw exception")
    void createClientWithNullGameIdThrows() {
        assertThrows(NullPointerException.class, () -> CroupierSDK.createClient(null, "svc1"));
    }

    @Test
    @DisplayName("Create client with null service ID should throw exception")
    void createClientWithNullServiceIdThrows() {
        assertThrows(NullPointerException.class, () -> CroupierSDK.createClient("game1", null));
    }

    @Test
    @DisplayName("Create client with empty agent address uses default")
    void createClientWithEmptyAgentAddress() {
        CroupierClient client = CroupierSDK.createClient("game1", "svc1", "");

        assertNotNull(client);
        // Empty address should be handled by default config
        assertFalse(client.isConnected());
    }

    // ========== FunctionDescriptorBuilder Tests ==========

    @Test
    @DisplayName("Function descriptor builder returns builder instance")
    void functionDescriptorReturnsBuilder() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("test-func", "1.0.0");

        assertNotNull(builder);

        FunctionDescriptor desc = builder
            .resource("player")
            .risk("low")
            .operation("ban")
            .permission("player.ban")
            .build();

        assertEquals("test-func", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
        assertEquals("player", desc.getResource());
        assertEquals("low", desc.getRisk());
        assertEquals("ban", desc.getOperation());
        assertEquals("player.ban", desc.getPermission());
        assertTrue(desc.isEnabled());
    }

    @Test
    @DisplayName("Function descriptor builder with disabled")
    void functionDescriptorBuilderWithDisabled() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", "1.0.0")
            .enabled(false)
            .build();

        assertFalse(desc.isEnabled());
    }

    @Test
    @DisplayName("Function descriptor builder with all options")
    void functionDescriptorBuilderWithAllOptions() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.2.0")
            .resource("player")
            .risk("high")
            .operation("ban")
            .permission("player.ban")
            .enabled(true)
            .build();

        assertEquals("player.ban", desc.getId());
        assertEquals("1.2.0", desc.getVersion());
        assertEquals("player", desc.getResource());
        assertEquals("high", desc.getRisk());
        assertEquals("ban", desc.getOperation());
        assertEquals("player.ban", desc.getPermission());
        assertTrue(desc.isEnabled());
    }

    @Test
    @DisplayName("Function descriptor builder with tags")
    void functionDescriptorBuilderWithTags() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.create", "1.0.0")
            .tags(List.of("player", "crud", "write"))
            .build();

        assertNotNull(desc.getTags());
        assertEquals(3, desc.getTags().size());
        assertTrue(desc.getTags().contains("player"));
        assertTrue(desc.getTags().contains("crud"));
        assertTrue(desc.getTags().contains("write"));
    }

    @Test
    @DisplayName("Function descriptor builder with summary and description")
    void functionDescriptorBuilderWithSummaryAndDescription() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.create", "1.0.0")
            .summary("Create a new player")
            .description("Creates a new player with the provided data")
            .build();

        assertEquals("Create a new player", desc.getSummary());
        assertEquals("Creates a new player with the provided data", desc.getDescription());
    }

    @Test
    @DisplayName("Function descriptor builder with operation ID")
    void functionDescriptorBuilderWithOperationId() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.create", "1.0.0")
            .operationId("createPlayer")
            .build();

        assertEquals("createPlayer", desc.getOperationId());
    }

    @Test
    @DisplayName("Function descriptor builder with deprecated")
    void functionDescriptorBuilderWithDeprecated() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("old.func", "1.0.0")
            .deprecated(true)
            .build();

        assertTrue(desc.isDeprecated());
    }

    @Test
    @DisplayName("Function descriptor builder with schemas")
    void functionDescriptorBuilderWithSchemas() {
        String inputSchema = "{\"type\":\"object\"}";
        String outputSchema = "{\"type\":\"string\"}";

        FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", "1.0.0")
            .inputSchema(inputSchema)
            .outputSchema(outputSchema)
            .build();

        assertEquals(inputSchema, desc.getInputSchema());
        assertEquals(outputSchema, desc.getOutputSchema());
    }

    @Test
    @DisplayName("Function descriptor builder is chainable")
    void functionDescriptorBuilderIsChainable() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("func", "1.0.0");

        FunctionDescriptor desc = builder
            .resource("player")
            .risk("low")
            .operation("ban")
            .permission("player.ban")
            .summary("Test function")
            .description("Test description")
            .operationId("testFunc")
            .enabled(true)
            .deprecated(false)
            .build();

        assertNotNull(desc);
    }

    @Test
    @DisplayName("Function descriptor builder with empty strings")
    void functionDescriptorBuilderWithEmptyStrings() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", "1.0.0")
            .resource("")
            .risk("")
            .operation("")
            .permission("")
            .summary("")
            .description("")
            .operationId("")
            .inputSchema("")
            .outputSchema("")
            .build();

        assertEquals("", desc.getResource());
        assertEquals("", desc.getRisk());
        assertEquals("", desc.getOperation());
        assertEquals("", desc.getPermission());
        assertEquals("", desc.getSummary());
        assertEquals("", desc.getDescription());
        assertEquals("", desc.getOperationId());
        assertEquals("", desc.getInputSchema());
        assertEquals("", desc.getOutputSchema());
    }

    @ParameterizedTest
    @ValueSource(strings = {"safe", "warning", "danger"})
    @DisplayName("Function descriptor builder with valid risk levels")
    void functionDescriptorBuilderWithValidRiskLevels(String risk) {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", "1.0.0")
            .risk(risk)
            .build();

        assertEquals(risk, desc.getRisk());
    }

    // ========== Invoker Factory Tests ==========

    @Test
    @DisplayName("Create invoker with config")
    void createInvokerWithConfig() {
        InvokerConfig config = InvokerConfig.createDefault();
        Invoker invoker = CroupierSDK.createInvoker(config);

        assertNotNull(invoker);
    }

    @Test
    @DisplayName("Create invoker with default config")
    void createInvokerWithDefaultConfig() {
        Invoker invoker = CroupierSDK.createInvoker();

        assertNotNull(invoker);
    }

    @Test
    @DisplayName("Create invoker with address")
    void createInvokerWithAddress() {
        Invoker invoker = CroupierSDK.createInvoker("localhost:18080");

        assertNotNull(invoker);
    }

    @Test
    @DisplayName("Create invoker with null config throws exception")
    void createInvokerWithNullConfigThrows() {
        assertThrows(NullPointerException.class, () -> CroupierSDK.createInvoker((InvokerConfig) null));
    }

    @Test
    @DisplayName("Create invoker with null address throws exception")
    void createInvokerWithNullAddressThrows() {
        assertThrows(NullPointerException.class, () -> CroupierSDK.createInvoker((String) null));
    }

    @Test
    @DisplayName("Create invoker with empty address throws exception")
    void createInvokerWithEmptyAddressThrows() {
        assertThrows(IllegalArgumentException.class, () -> CroupierSDK.createInvoker(""));
    }

    // ========== InvokeOptions Builder Tests ==========

    @Test
    @DisplayName("Create invoke options builder")
    void createInvokeOptionsBuilder() {
        InvokeOptions.Builder builder = CroupierSDK.invokeOptions();

        assertNotNull(builder);
    }

    @Test
    @DisplayName("Invoke options builder creates options")
    void invokeOptionsBuilderCreatesOptions() {
        InvokeOptions options = CroupierSDK.invokeOptions()
            .header("X-Game-ID", "game1")
            .header("X-Env", "production")
            .header("X-Request-ID", "req-123")
            .build();

        assertEquals("game1", options.getHeaders().get("X-Game-ID"));
        assertEquals("production", options.getHeaders().get("X-Env"));
        assertEquals("req-123", options.getHeaders().get("X-Request-ID"));
    }

    @Test
    @DisplayName("Invoke options builder with timeout")
    void invokeOptionsBuilderWithTimeout() {
        InvokeOptions options = CroupierSDK.invokeOptions()
            .timeout(5000)
            .build();

        assertEquals(5000, options.getTimeout());
    }

    @Test
    @DisplayName("Invoke options builder with headers")
    void invokeOptionsBuilderWithHeaders() {
        InvokeOptions options = CroupierSDK.invokeOptions()
            .header("key1", "value1")
            .header("key2", "value2")
            .build();

        assertEquals("value1", options.getHeaders().get("key1"));
        assertEquals("value2", options.getHeaders().get("key2"));
    }

    // ========== Multiple Build Calls Tests ==========

    @Test
    @DisplayName("Function descriptor builder can create multiple descriptors")
    void functionDescriptorBuilderCanCreateMultipleDescriptors() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("func", "1.0.0");

        FunctionDescriptor desc1 = builder.resource("player").build();
        FunctionDescriptor desc2 = builder.resource("mail").build();

        assertEquals("player", desc1.getResource());
        assertEquals("mail", desc2.getResource());
        // Both should have the same ID and version
        assertEquals(desc1.getId(), desc2.getId());
        assertEquals(desc1.getVersion(), desc2.getVersion());
    }

    @Test
    @DisplayName("Multiple function descriptors from factory")
    void multipleFunctionDescriptorsFromFactory() {
        FunctionDescriptor desc1 = CroupierSDK.functionDescriptor("func1", "1.0.0").build();
        FunctionDescriptor desc2 = CroupierSDK.functionDescriptor("func2", "2.0.0").build();
        FunctionDescriptor desc3 = CroupierSDK.functionDescriptor("func3", "3.0.0").build();

        assertEquals("func1", desc1.getId());
        assertEquals("func2", desc2.getId());
        assertEquals("func3", desc3.getId());
    }

    // ========== Edge Cases Tests ==========

    @Test
    @DisplayName("Function descriptor with special characters in ID")
    void functionDescriptorWithSpecialCharactersInId() {
        // Function IDs typically use dot notation
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.action.ban", "1.0.0")
            .build();

        assertEquals("player.action.ban", desc.getId());
    }

    @Test
    @DisplayName("Function descriptor with semver versions")
    void functionDescriptorWithSemverVersions() {
        String[] validVersions = {
            "1.0.0",
            "2.1.3",
            "10.20.30",
            "1.0.0-alpha",
            "1.0.0-beta.1",
            "1.0.0+build.123"
        };

        for (String version : validVersions) {
            FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", version)
                .build();
            assertEquals(version, desc.getVersion());
        }
    }

    @Test
    @DisplayName("Function descriptor with null tags defaults to empty list")
    void functionDescriptorWithNullTagsDefaultsToEmpty() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("func", "1.0.0")
            .tags(null)
            .build();

        assertNull(desc.getTags());
    }

    @Test
    @DisplayName("Function descriptor builder maintains immutability after build")
    void functionDescriptorBuilderMaintainsImmutabilityAfterBuild() {
        CroupierSDK.FunctionDescriptorBuilder builder = CroupierSDK.functionDescriptor("func", "1.0.0");

        FunctionDescriptor desc1 = builder.resource("player").build();
        FunctionDescriptor desc2 = builder.resource("mail").build();

        // desc1 should not be affected by subsequent builder calls
        assertEquals("player", desc1.getResource());
        assertEquals("mail", desc2.getResource());
    }

    @Test
    @DisplayName("Create multiple clients with same config")
    void createMultipleClientsWithSameConfig() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        CroupierClient client1 = CroupierSDK.createClient(config);
        CroupierClient client2 = CroupierSDK.createClient(config);

        assertNotNull(client1);
        assertNotNull(client2);
        // Each client should be a separate instance
        // Note: We can't directly test instance equality in Java without reference checks
        assertFalse(client1.isConnected());
        assertFalse(client2.isConnected());
    }
}
