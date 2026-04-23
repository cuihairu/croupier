package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.concurrent.CompletableFuture;
import org.junit.jupiter.api.Test;

class CroupierClientImplConfigTest {

    @Test
    void clientWithProductionEnvironment() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnv("production");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("production", config.getEnv());
    }

    @Test
    void clientWithStagingEnvironment() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnv("staging");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("staging", config.getEnv());
    }

    @Test
    void clientWithCustomServiceVersion() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setServiceVersion("2.5.0");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("2.5.0", config.getServiceVersion());
    }

    @Test
    void clientWithControlAddress() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setControlAddr("localhost:8080");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("localhost:8080", config.getControlAddr());
    }

    @Test
    void clientWithTimeout() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setTimeoutSeconds(120);
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals(120, config.getTimeoutSeconds());
    }

    @Test
    void clientWithLocalListen() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setLocalListen("0.0.0.0:9999");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("0.0.0.0:9999", config.getLocalListen());
    }

    @Test
    void clientWithTlsSettings() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setInsecure(false);
        config.setCaFile("/ca.pem");
        config.setCertFile("/cert.pem");
        config.setKeyFile("/key.pem");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertFalse(config.isInsecure());
    }

    @Test
    void clientWithAgentId() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setAgentId("agent-001");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("agent-001", config.getAgentId());
    }

    @Test
    void clientWithProviderMetadata() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setProviderLang("java");
        config.setProviderSdk("custom-sdk");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("java", config.getProviderLang());
        assertEquals("custom-sdk", config.getProviderSdk());
    }

    @Test
    void clientWithEmptyGameId() {
        ClientConfig config = new ClientConfig("", "svc1");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    void clientWithWhitespaceGameId() {
        ClientConfig config = new ClientConfig("   ", "svc1");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertFalse(client.isConnected());
    }

    @Test
    void clientWithAllConfigurationOptions() {
        ClientConfig config = new ClientConfig("test-game", "test-svc");
        config.setAgentAddr("localhost:19090");
        config.setLocalListen("127.0.0.1:8888");
        config.setEnv("production");
        config.setServiceVersion("3.0.0");
        config.setAgentId("agent-prod-01");
        config.setControlAddr("control.example.com:8080");
        config.setTimeoutSeconds(60);
        config.setInsecure(false);
        config.setCaFile("/etc/ssl/ca.pem");
        config.setCertFile("/etc/ssl/cert.pem");
        config.setKeyFile("/etc/ssl/key.pem");
        config.setProviderLang("java");
        config.setProviderSdk("croupier-sdk-java");

        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertFalse(client.isConnected());
        assertFalse(client.isServing());
        assertEquals("test-game", config.getGameId());
        assertEquals("test-svc", config.getServiceId());
        assertEquals("production", config.getEnv());
    }

    @Test
    void stopMultipleClients() {
        ClientConfig config1 = new ClientConfig("game1", "svc1");
        ClientConfig config2 = new ClientConfig("game2", "svc2");
        CroupierClientImpl client1 = new CroupierClientImpl(config1);
        CroupierClientImpl client2 = new CroupierClientImpl(config2);

        assertDoesNotThrow(() -> {
            client1.stop();
            client2.stop();
        });
    }

    @Test
    void closeMultipleClients() {
        ClientConfig config1 = new ClientConfig("game1", "svc1");
        ClientConfig config2 = new ClientConfig("game2", "svc2");
        CroupierClientImpl client1 = new CroupierClientImpl(config1);
        CroupierClientImpl client2 = new CroupierClientImpl(config2);

        assertDoesNotThrow(() -> {
            client1.close();
            client2.close();
        });
    }

    @Test
    void connectAlwaysReturnsFuture() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        CroupierClientImpl client = new CroupierClientImpl(config);

        CompletableFuture<Void> future = client.connect();
        assertNotNull(future);
    }

    @Test
    void serveAsyncAlwaysReturnsFuture() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        CroupierClientImpl client = new CroupierClientImpl(config);

        CompletableFuture<Void> future = client.serveAsync();
        assertNotNull(future);
    }

    @Test
    void defaultConfigValues() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertEquals("development", config.getEnv());
        assertEquals("1.0.0", config.getServiceVersion());
        assertEquals("127.0.0.1:19090", config.getAgentAddr());
        assertEquals(30, config.getTimeoutSeconds());
        assertTrue(config.isInsecure());
        assertEquals("java", config.getProviderLang());
        assertEquals("croupier-java-sdk", config.getProviderSdk());
    }

    @Test
    void registerFunctionOverwritesPrevious() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertDoesNotThrow(() -> {
            client.registerFunction(new FunctionDescriptor("func1", "1.0.0"), (ctx, payload) -> "v1");
            client.registerFunction(new FunctionDescriptor("func1", "2.0.0"), (ctx, payload) -> "v2");
        });
    }
}
