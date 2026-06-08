package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.Invoker;
import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for CroupierSDK factory methods and FunctionDescriptorBuilder.
 */
class CroupierSDKCoverageTest {

    @Test
    @DisplayName("createClient(config) should return non-null")
    void createClientWithConfig() {
        ClientConfig config = new ClientConfig("game", "svc");
        CroupierClient client = CroupierSDK.createClient(config);
        assertNotNull(client);
    }

    @Test
    @DisplayName("createClient(gameId, serviceId) should return non-null")
    void createClientWithStrings() {
        CroupierClient client = CroupierSDK.createClient("game", "svc");
        assertNotNull(client);
    }

    @Test
    @DisplayName("createClient with null gameId should throw")
    void createClientNullGameId() {
        assertThrows(NullPointerException.class, () ->
            CroupierSDK.createClient(null, "svc"));
    }

    @Test
    @DisplayName("createClient with null serviceId should throw")
    void createClientNullServiceId() {
        assertThrows(NullPointerException.class, () ->
            CroupierSDK.createClient("game", null));
    }

    @Test
    @DisplayName("createClient(gameId, serviceId, agentAddr) should return non-null")
    void createClientWithAgentAddr() {
        CroupierClient client = CroupierSDK.createClient("game", "svc", "10.0.0.1:19090");
        assertNotNull(client);
    }

    @Test
    @DisplayName("createClient(gameId, serviceId, agentAddr) with null gameId should throw")
    void createClientWithAgentAddrNullGameId() {
        assertThrows(NullPointerException.class, () ->
            CroupierSDK.createClient(null, "svc", "addr"));
    }

    @Test
    @DisplayName("createClient(gameId, serviceId, agentAddr) with null serviceId should throw")
    void createClientWithAgentAddrNullServiceId() {
        assertThrows(NullPointerException.class, () ->
            CroupierSDK.createClient("game", null, "addr"));
    }

    @Test
    @DisplayName("createInvoker(config) should return non-null")
    void createInvokerWithConfig() {
        Invoker invoker = CroupierSDK.createInvoker(InvokerConfig.createDefault());
        assertNotNull(invoker);
    }

    @Test
    @DisplayName("createInvoker(config) with null should throw")
    void createInvokerNullConfig() {
        assertThrows(NullPointerException.class, () ->
            CroupierSDK.createInvoker((InvokerConfig) null));
    }

    @Test
    @DisplayName("createInvoker() should return non-null with defaults")
    void createInvokerDefault() {
        Invoker invoker = CroupierSDK.createInvoker();
        assertNotNull(invoker);
    }

    @Test
    @DisplayName("createInvoker(address) should return non-null")
    void createInvokerWithAddress() {
        Invoker invoker = CroupierSDK.createInvoker("10.0.0.1:19090");
        assertNotNull(invoker);
    }

    @Test
    @DisplayName("createInvoker(address) with null should throw")
    void createInvokerNullAddress() {
        assertThrows(NullPointerException.class, () ->
            CroupierSDK.createInvoker((String) null));
    }

    @Test
    @DisplayName("createInvoker(address) with empty should throw")
    void createInvokerEmptyAddress() {
        assertThrows(IllegalArgumentException.class, () ->
            CroupierSDK.createInvoker(""));
    }

    @Test
    @DisplayName("invokeOptions() should return builder")
    void invokeOptionsBuilder() {
        assertNotNull(CroupierSDK.invokeOptions());
    }

    @Test
    @DisplayName("functionDescriptor builder should build with all fields")
    void functionDescriptorBuilder() {
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("f1", "1.0.0")
            .category("game")
            .tags(List.of("tag1", "tag2"))
            .summary("Summary")
            .description("Description")
            .operationId("opId")
            .deprecated(true)
            .inputSchema("{\"type\":\"object\"}")
            .outputSchema("{\"type\":\"string\"}")
            .risk("low")
            .entity("Player")
            .operation("create")
            .enabled(true)
            .build();

        assertEquals("f1", desc.getId());
        assertEquals("1.0.0", desc.getVersion());
        assertEquals("game", desc.getCategory());
        assertEquals(2, desc.getTags().size());
        assertEquals("Summary", desc.getSummary());
        assertEquals("Description", desc.getDescription());
        assertEquals("opId", desc.getOperationId());
        assertTrue(desc.isDeprecated());
        assertEquals("{\"type\":\"object\"}", desc.getInputSchema());
        assertEquals("{\"type\":\"string\"}", desc.getOutputSchema());
        assertEquals("low", desc.getRisk());
        assertEquals("Player", desc.getEntity());
        assertEquals("create", desc.getOperation());
        assertTrue(desc.isEnabled());
    }

    @Test
    @DisplayName("ReconnectConfig.Builder with all fields")
    void reconnectConfigBuilder() {
        ReconnectConfig config = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(10)
            .initialDelayMs(500)
            .maxDelayMs(30000)
            .build();

        assertTrue(config.isEnabled());
        assertEquals(10, config.getMaxAttempts());
        assertEquals(500, config.getInitialDelayMs());
        assertEquals(30000, config.getMaxDelayMs());
    }

    @Test
    @DisplayName("ReconnectConfig.createDefault() should return defaults")
    void reconnectConfigDefault() {
        ReconnectConfig config = ReconnectConfig.createDefault();
        assertNotNull(config);
    }

    @Test
    @DisplayName("ReconnectConfig toString should include fields")
    void reconnectConfigToString() {
        ReconnectConfig config = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(5)
            .build();

        String str = config.toString();
        assertTrue(str.contains("ReconnectConfig"));
    }
}
